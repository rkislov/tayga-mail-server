package httpapi

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"github.com/tayga/tms/internal/avatar"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
)

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	id := au.ID
	if email := r.URL.Query().Get("email"); email != "" {
		if r.Method != http.MethodGet {
			writeJSON(w, 403, map[string]string{"error": "owner required"})
			return
		}
		other, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(email)))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "avatar not found"})
			return
		}
		me, err := s.store.GetUserByID(r.Context(), au.ID)
		if err != nil || me.TenantID != other.TenantID {
			writeJSON(w, 404, map[string]string{"error": "avatar not found"})
			return
		}
		id = other.ID
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch r.Method {
	case http.MethodGet:
		value, err := avatar.Get(r.Context(), s.store, id)
		if err != nil || value.Data == "" {
			writeJSON(w, 404, map[string]string{"error": "avatar not found"})
			return
		}
		raw, err := base64.StdEncoding.DecodeString(value.Data)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "invalid avatar"})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(raw)
	case http.MethodPut:
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			writeJSON(w, 413, map[string]string{"error": "avatar exceeds 2 MiB"})
			return
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 {
			writeJSON(w, 400, map[string]string{"error": "PNG or JPEG up to 4096 pixels required"})
			return
		}
		source, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid image"})
			return
		}
		size := 256
		dest := image.NewRGBA(image.Rect(0, 0, size, size))
		bounds := source.Bounds()
		crop := bounds.Dx()
		if bounds.Dy() < crop {
			crop = bounds.Dy()
		}
		x, y := bounds.Min.X+(bounds.Dx()-crop)/2, bounds.Min.Y+(bounds.Dy()-crop)/2
		for dy := 0; dy < size; dy++ {
			for dx := 0; dx < size; dx++ {
				dest.Set(dx, dy, source.At(x+dx*crop/size, y+dy*crop/size))
			}
		}
		var buf bytes.Buffer
		if err = png.Encode(&buf, dest); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot encode image"})
			return
		}
		sum := sha1.Sum(buf.Bytes())
		value := avatar.Image{Data: base64.StdEncoding.EncodeToString(buf.Bytes()), Hash: hex.EncodeToString(sum[:])}
		if err = avatar.Save(r.Context(), s.store, id, value); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot save avatar"})
			return
		}
		writeJSON(w, 200, map[string]string{"hash": value.Hash})
	case http.MethodDelete:
		if err = avatar.Save(r.Context(), s.store, id, avatar.Image{}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot remove avatar"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
