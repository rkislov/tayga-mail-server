// Package mailcharset provides the same legacy mail decoders to MIME readers,
// header decoding and IMAP. Decoding is for views and indexes only; stored
// RFC822 messages must retain their original bytes.
package mailcharset

import (
	"io"
	"strings"

	"github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/charmap"
)

func init() {
	// Non-IANA spellings used by older Windows mail clients. Registration also
	// makes these aliases available to go-message (FlowSync and IMAP SEARCH).
	charset.RegisterEncoding("win-1251", charmap.Windows1251)
	charset.RegisterEncoding("win1251", charmap.Windows1251)
}

// Reader converts an explicitly declared charset to UTF-8. Unknown charsets
// return an error rather than guessing a potentially destructive conversion.
func Reader(label string, input io.Reader) (io.Reader, error) {
	return charset.Reader(strings.ToLower(strings.TrimSpace(label)), input)
}
