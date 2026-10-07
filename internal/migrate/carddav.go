package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
	"github.com/tayga/tms/internal/storage"
)

type cardOptions struct {
	AddressBookPath string `json:"addressbook_path"`
	LocalName       string `json:"local_name"`
}

func (s *Service) runCardDAV(ctx context.Context, job *Job) error {
	password, err := decryptPassword(s.key, job.PasswordCiphertext)
	if err != nil {
		return err
	}
	user, err := s.store.GetUserByID(ctx, job.UserID)
	if err != nil {
		return err
	}
	if err := s.store.EnsureDAVDefaults(ctx, user.ID); err != nil {
		return err
	}

	var opts cardOptions
	_ = json.Unmarshal([]byte(job.Options), &opts)
	localName := strings.TrimSpace(opts.LocalName)
	if localName == "" {
		localName = "default"
	}

	httpClient := webdav.HTTPClientWithBasicAuth(&http.Client{Timeout: 60 * time.Second}, job.Username, password)
	cli, err := carddav.NewClient(httpClient, job.URL)
	if err != nil {
		return err
	}
	principal, err := cli.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return fmt.Errorf("principal: %w", err)
	}
	home, err := cli.FindAddressBookHomeSet(ctx, principal)
	if err != nil {
		return fmt.Errorf("addressbook-home: %w", err)
	}
	books, err := cli.FindAddressBooks(ctx, home)
	if err != nil {
		return fmt.Errorf("find addressbooks: %w", err)
	}

	dbBook, err := s.store.EnsureAddressBook(ctx, user.ID, localName, localName)
	if err != nil {
		return err
	}

	copied, skipped, errs := job.Copied, job.Skipped, job.Errors
	query := &carddav.AddressBookQuery{
		DataRequest: carddav.AddressDataRequest{AllProp: true},
	}

	for _, book := range books {
		if opts.AddressBookPath != "" && !strings.Contains(book.Path, opts.AddressBookPath) {
			continue
		}
		if ctx.Err() != nil || s.isCancelled(ctx, job.ID) {
			job.Copied, job.Skipped, job.Errors = copied, skipped, errs
			return ctx.Err()
		}
		objs, err := cli.QueryAddressBook(ctx, book.Path, query)
		if err != nil {
			errs++
			_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, err.Error(), `{"book":`+jsonString(book.Path)+`}`)
			continue
		}
		for _, obj := range objs {
			if ctx.Err() != nil || s.isCancelled(ctx, job.ID) {
				job.Copied, job.Skipped, job.Errors = copied, skipped, errs
				return ctx.Err()
			}
			if len(obj.Card) == 0 {
				continue
			}
			uid := obj.Card.Value(vcard.FieldUID)
			if uid == "" {
				uid = storage.NewID()
				obj.Card.SetValue(vcard.FieldUID, uid)
			}
			exists, err := s.store.AddressObjectExistsByUID(ctx, dbBook.ID, uid)
			if err == nil && exists {
				skipped++
				continue
			}
			var buf bytes.Buffer
			if err := vcard.NewEncoder(&buf).Encode(obj.Card); err != nil {
				errs++
				continue
			}
			href := path.Base(strings.TrimSuffix(obj.Path, "/"))
			if href == "" || href == "." || href == "/" {
				href = uid + ".vcf"
			}
			_, err = s.store.UpsertAddressObject(ctx, &storage.AddressObject{
				AddressBookID: dbBook.ID,
				UID:           uid,
				HrefName:      href,
				Data:          buf.String(),
			})
			if err != nil {
				errs++
				continue
			}
			copied++
			if (copied+skipped+errs)%20 == 0 {
				_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", `{"book":`+jsonString(book.Path)+`}`)
			}
		}
		_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", `{"book":`+jsonString(book.Path)+`}`)
	}
	job.Copied, job.Skipped, job.Errors = copied, skipped, errs
	_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", "{}")
	return nil
}
