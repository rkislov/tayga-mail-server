package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleContacts(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/contacts"), "/")
	parts := splitPath(path)

	switch {
	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "books" && parts[1] == "gal" && parts[2] == "cards":
		s.handleGAL(w, r, au, "")
	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "cards" && strings.HasPrefix(parts[1], "gal-"):
		s.handleGAL(w, r, au, parts[1])

	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "books":
		_ = s.store.EnsureDAVDefaults(r.Context(), au.ID)
		books, err := s.store.ListAddressBooks(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(books))
		for _, b := range books {
			out = append(out, map[string]any{
				"id": b.ID, "name": b.Name, "display_name": b.DisplayName,
			})
		}
		current, err := s.store.GetUserByID(r.Context(), au.ID)
		if err == nil && s.domainGALEnabled(r, current.DomainID) {
			out = append(out, map[string]any{"id": "gal", "name": "gal", "display_name": "GAL", "readonly": true})
		}
		writeJSON(w, http.StatusOK, map[string]any{"books": out})

	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "books" && parts[2] == "cards":
		book, err := s.store.GetAddressBookByID(r.Context(), au.ID, parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		cards, err := s.store.ListAddressObjects(r.Context(), book.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(cards))
		for _, c := range cards {
			v := parseSimpleVCard(c.Data)
			out = append(out, map[string]any{
				"id": c.ID, "book_id": book.ID, "uid": c.UID,
				"fn": v.FN, "email": v.Email, "tel": v.Tel, "org": v.Org,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"cards": out})

	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "cards":
		c, err := s.store.GetAddressObjectByID(r.Context(), parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if _, err := s.store.GetAddressBookByID(r.Context(), au.ID, c.AddressBookID); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		v := parseSimpleVCard(c.Data)
		writeJSON(w, http.StatusOK, map[string]any{
			"id": c.ID, "book_id": c.AddressBookID, "uid": c.UID,
			"fn": v.FN, "email": v.Email, "tel": v.Tel, "org": v.Org, "note": v.Note, "vcard": c.Data,
		})

	case (r.Method == http.MethodPost || r.Method == http.MethodPut) && len(parts) == 3 && parts[0] == "books" && parts[2] == "cards":
		book, err := s.store.GetAddressBookByID(r.Context(), au.ID, parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			ID    string `json:"id"`
			UID   string `json:"uid"`
			FN    string `json:"fn"`
			Email string `json:"email"`
			Tel   string `json:"tel"`
			Org   string `json:"org"`
			Note  string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(req.FN) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fn required"})
			return
		}
		oldHref := ""
		if req.ID != "" {
			old, e := s.store.GetAddressObjectByID(r.Context(), req.ID)
			if e != nil || old.AddressBookID != book.ID {
				writeJSON(w, 403, map[string]string{"error": "forbidden"})
				return
			}
			req.UID = old.UID
			oldHref = old.HrefName
		}
		uid := req.UID
		if uid == "" {
			uid = storage.NewID()
		}
		href := uid + ".vcf"
		if oldHref != "" {
			href = oldHref
		}
		data := buildSimpleVCARD(uid, req.FN, req.Email, req.Tel, req.Org, req.Note)
		obj := &storage.AddressObject{
			AddressBookID: book.ID, UID: uid, HrefName: href,
			Data: data, Size: int64(len(data)), ETag: storage.ContentETag(data),
		}
		if req.ID != "" {
			obj.ID = req.ID
		}
		saved, err := s.store.UpsertAddressObject(r.Context(), obj)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": saved.ID, "uid": saved.UID, "book_id": book.ID})

	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "cards":
		c, err := s.store.GetAddressObjectByID(r.Context(), parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if _, err := s.store.GetAddressBookByID(r.Context(), au.ID, c.AddressBookID); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if err := s.store.DeleteAddressObject(r.Context(), c.AddressBookID, c.HrefName); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

type simpleCard struct {
	FN, Email, Tel, Org, Note string
}

func parseSimpleVCard(data string) simpleCard {
	c := simpleCard{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "FN:"):
			c.FN = icsUnescape(line[3:])
		case strings.HasPrefix(upper, "EMAIL"):
			if i := strings.IndexByte(line, ':'); i >= 0 && c.Email == "" {
				c.Email = strings.TrimSpace(line[i+1:])
			}
		case strings.HasPrefix(upper, "TEL"):
			if i := strings.IndexByte(line, ':'); i >= 0 && c.Tel == "" {
				c.Tel = strings.TrimSpace(line[i+1:])
			}
		case strings.HasPrefix(upper, "ORG:"):
			c.Org = icsUnescape(line[4:])
		case strings.HasPrefix(upper, "NOTE:"):
			c.Note = icsUnescape(line[5:])
		}
	}
	return c
}

func buildSimpleVCARD(uid, fn, email, tel, org, note string) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCARD\r\nVERSION:3.0\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	fmt.Fprintf(&b, "FN:%s\r\n", icsEscape(fn))
	if email != "" {
		fmt.Fprintf(&b, "EMAIL;TYPE=INTERNET:%s\r\n", icsEscape(email))
	}
	if tel != "" {
		fmt.Fprintf(&b, "TEL:%s\r\n", icsEscape(tel))
	}
	if org != "" {
		fmt.Fprintf(&b, "ORG:%s\r\n", icsEscape(org))
	}
	if note != "" {
		fmt.Fprintf(&b, "NOTE:%s\r\n", icsEscape(note))
	}
	b.WriteString("END:VCARD\r\n")
	return b.String()
}
