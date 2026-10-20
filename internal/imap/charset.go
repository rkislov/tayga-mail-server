package imapserver

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-imap"
	imapserv "github.com/emersion/go-imap/server"
	"github.com/tayga/tms/internal/mailcharset"
)

func init() {
	// go-imap uses a separate reader for ENVELOPE and SEARCH CHARSET. Without
	// it, legacy search criteria can panic and encoded subjects stay unreadable.
	imap.CharsetReader = mailcharset.Reader
}

type charsetSearchExtension struct{}

func (charsetSearchExtension) Capabilities(imapserv.Conn) []string { return nil }
func (charsetSearchExtension) Command(name string) imapserv.HandlerFactory {
	if name == "SEARCH" {
		return func() imapserv.Handler { return new(charsetSearch) }
	}
	return nil
}

type charsetSearch struct {
	imapserv.Search
	charsetErr error
}

func (cmd *charsetSearch) Parse(fields []interface{}) error {
	if len(fields) < 2 {
		return cmd.Search.Parse(fields)
	}
	key, _ := fields[0].(string)
	label, _ := fields[1].(string)
	if !strings.EqualFold(key, "CHARSET") || label == "" || strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "us-ascii") {
		return cmd.Search.Parse(fields)
	}
	if _, err := mailcharset.Reader(label, strings.NewReader("")); err != nil {
		cmd.charsetErr = &imap.ErrStatusResp{Resp: &imap.StatusResp{Type: imap.StatusRespNo, Code: imap.CodeBadCharset, Info: "Unsupported search charset"}}
		return nil
	}
	// go-imap v1 reads only literal.Len() bytes *after* conversion. Single-byte
	// Cyrillic expands in UTF-8, truncating search terms. Convert literals first
	// and let the stock parser see their actual UTF-8 byte lengths.
	fields = append([]interface{}(nil), fields...)
	fields[1] = "UTF-8"
	budget := int64(1 << 20)
	var convert func([]interface{}, int) error
	convert = func(values []interface{}, depth int) error {
		if depth > 20 {
			return fmt.Errorf("search criteria nested too deeply")
		}
		for i, value := range values {
			switch v := value.(type) {
			case []interface{}:
				if err := convert(v, depth+1); err != nil {
					return err
				}
			case imap.Literal:
				r, err := mailcharset.Reader(label, v)
				if err != nil {
					return err
				}
				data, err := io.ReadAll(io.LimitReader(r, budget+1))
				if err != nil {
					return err
				}
				budget -= int64(len(data))
				if budget < 0 {
					return fmt.Errorf("search criteria too large")
				}
				values[i] = bytes.NewReader(data)
			}
		}
		return nil
	}
	if err := convert(fields[2:], 0); err != nil {
		return err
	}
	return cmd.Search.Parse(fields)
}

func (cmd *charsetSearch) Handle(conn imapserv.Conn) error {
	if cmd.charsetErr != nil {
		return cmd.charsetErr
	}
	return cmd.Search.Handle(conn)
}

func (cmd *charsetSearch) UidHandle(conn imapserv.Conn) error {
	if cmd.charsetErr != nil {
		return cmd.charsetErr
	}
	return cmd.Search.UidHandle(conn)
}
