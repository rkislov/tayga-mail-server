package dav

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
	"github.com/tayga/tms/internal/storage"
)

type cardBackend struct {
	store storage.Driver
}

func (b *cardBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return "", webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	return cardPrincipal(u.Email), nil
}

func (b *cardBackend) AddressBookHomeSetPath(ctx context.Context) (string, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return "", webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	return cardHome(u.Email), nil
}

func (b *cardBackend) CreateAddressBook(ctx context.Context, addressBook *carddav.AddressBook) error {
	u, ok := userFrom(ctx)
	if !ok {
		return webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, name, href, okParse := parseCardPath(addressBook.Path)
	if !okParse || href != "" || name == "" {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("invalid addressbook path"))
	}
	if !strings.EqualFold(email, u.Email) {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	display := addressBook.Name
	if display == "" {
		display = name
	}
	_, err := b.store.CreateAddressBook(ctx, &storage.AddressBook{
		UserID:      u.ID,
		Name:        name,
		DisplayName: display,
		Description: addressBook.Description,
	})
	if err != nil {
		return webdav.NewHTTPError(http.StatusConflict, err)
	}
	return nil
}

func (b *cardBackend) DeleteAddressBook(ctx context.Context, p string) error {
	u, ok := userFrom(ctx)
	if !ok {
		return webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, name, href, okParse := parseCardPath(p)
	if !okParse || name == "" || href != "" {
		return webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	if name == "default" {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("cannot delete default addressbook"))
	}
	if err := b.store.DeleteAddressBook(ctx, u.ID, name); err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	return nil
}

func (b *cardBackend) ListAddressBooks(ctx context.Context) ([]carddav.AddressBook, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	rows, err := b.store.ListAddressBooks(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	out := make([]carddav.AddressBook, 0, len(rows))
	for _, ab := range rows {
		out = append(out, toCardDAVAddressBook(u.Email, ab))
	}
	return out, nil
}

func (b *cardBackend) GetAddressBook(ctx context.Context, p string) (*carddav.AddressBook, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, name, href, okParse := parseCardPath(p)
	if !okParse || name == "" || href != "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	ab, err := b.store.GetAddressBookByName(ctx, u.ID, name)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	out := toCardDAVAddressBook(u.Email, ab)
	return &out, nil
}

func (b *cardBackend) GetAddressObject(ctx context.Context, p string, req *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, abName, href, okParse := parseCardPath(p)
	if !okParse || abName == "" || href == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	ab, err := b.store.GetAddressBookByName(ctx, u.ID, abName)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	obj, err := b.store.GetAddressObject(ctx, ab.ID, href)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	ao, err := toCardDAVObject(u.Email, abName, obj)
	if err != nil {
		return nil, err
	}
	if req != nil {
		filtered, err := carddav.Filter(&carddav.AddressBookQuery{DataRequest: *req}, []carddav.AddressObject{*ao})
		if err != nil {
			return nil, err
		}
		if len(filtered) == 1 {
			return &filtered[0], nil
		}
	}
	return ao, nil
}

func (b *cardBackend) ListAddressObjects(ctx context.Context, p string, req *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, abName, href, okParse := parseCardPath(p)
	if !okParse || abName == "" || href != "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	ab, err := b.store.GetAddressBookByName(ctx, u.ID, abName)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	rows, err := b.store.ListAddressObjects(ctx, ab.ID)
	if err != nil {
		return nil, err
	}
	out := make([]carddav.AddressObject, 0, len(rows))
	for _, row := range rows {
		ao, err := toCardDAVObject(u.Email, abName, row)
		if err != nil {
			return nil, err
		}
		out = append(out, *ao)
	}
	if req != nil {
		return carddav.Filter(&carddav.AddressBookQuery{DataRequest: *req}, out)
	}
	return out, nil
}

func (b *cardBackend) QueryAddressObjects(ctx context.Context, p string, query *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	var req *carddav.AddressDataRequest
	if query != nil {
		req = &query.DataRequest
	}
	objs, err := b.ListAddressObjects(ctx, p, req)
	if err != nil {
		return nil, err
	}
	return carddav.Filter(query, objs)
}

func (b *cardBackend) PutAddressObject(ctx context.Context, p string, card vcard.Card, opts *carddav.PutAddressObjectOptions) (*carddav.AddressObject, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, abName, href, okParse := parseCardPath(p)
	if !okParse || abName == "" {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("invalid path"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	if href == "" {
		href = storage.NewID() + ".vcf"
	}
	uid := card.Value(vcard.FieldUID)
	if uid == "" {
		uid = storage.NewID()
		card.SetValue(vcard.FieldUID, uid)
	}

	ab, err := b.store.GetAddressBookByName(ctx, u.ID, abName)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	existing, err := b.store.GetAddressObject(ctx, ab.ID, href)
	if err != nil && err != storage.ErrNotFound {
		return nil, err
	}
	if err == storage.ErrNotFound {
		existing = nil
	}
	if err := checkCardConditions(opts, existing); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
	}
	saved, err := b.store.UpsertAddressObject(ctx, &storage.AddressObject{
		AddressBookID: ab.ID,
		UID:           uid,
		HrefName:      href,
		Data:          buf.String(),
	})
	if err != nil {
		return nil, err
	}
	return toCardDAVObject(u.Email, abName, saved)
}

func (b *cardBackend) DeleteAddressObject(ctx context.Context, p string) error {
	u, ok := userFrom(ctx)
	if !ok {
		return webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, abName, href, okParse := parseCardPath(p)
	if !okParse || abName == "" || href == "" {
		return webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	ab, err := b.store.GetAddressBookByName(ctx, u.ID, abName)
	if err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	if err := b.store.DeleteAddressObject(ctx, ab.ID, href); err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	return nil
}

func toCardDAVAddressBook(email string, ab *storage.AddressBook) carddav.AddressBook {
	return carddav.AddressBook{
		Path:        cardCollection(email, ab.Name),
		Name:        ab.DisplayName,
		Description: ab.Description,
		SupportedAddressData: []carddav.AddressDataType{
			{ContentType: "text/vcard", Version: "3.0"},
			{ContentType: "text/vcard", Version: "4.0"},
		},
	}
}

func toCardDAVObject(email, abName string, o *storage.AddressObject) (*carddav.AddressObject, error) {
	card, err := vcard.NewDecoder(strings.NewReader(o.Data)).Decode()
	if err != nil {
		return nil, err
	}
	return &carddav.AddressObject{
		Path:          cardObjectPath(email, abName, o.HrefName),
		ModTime:       o.UpdatedAt,
		ContentLength: o.Size,
		ETag:          o.ETag,
		Card:          card,
	}, nil
}

func checkCardConditions(opts *carddav.PutAddressObjectOptions, existing *storage.AddressObject) error {
	if opts == nil {
		return nil
	}
	if opts.IfNoneMatch.IsSet() {
		if opts.IfNoneMatch.IsWildcard() && existing != nil {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("resource exists"))
		}
	}
	if opts.IfMatch.IsSet() {
		if existing == nil {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("resource missing"))
		}
		ok, err := opts.IfMatch.MatchETag(existing.ETag)
		if err != nil || !ok {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("etag mismatch"))
		}
	}
	return nil
}
