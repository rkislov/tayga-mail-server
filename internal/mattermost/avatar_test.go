package mattermost

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"net/http/httptest"
	"testing"

	"github.com/tayga/tms/internal/avatar"
	"github.com/tayga/tms/internal/chat"
)

func TestProfileImageInitialsAndUploadedAvatar(t *testing.T) {
	f := setup(t)
	request := func(id, token string, status int) []byte {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v4/users/"+id+"/image", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		f.api.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("image status %d: %s", w.Code, w.Body.String())
		}
		if status == 200 {
			if w.Header().Get("Content-Type") != "image/png" {
				t.Fatal("native client requires PNG")
			}
			if _, err := png.Decode(bytes.NewReader(w.Body.Bytes())); err != nil {
				t.Fatal(err)
			}
		}
		return w.Body.Bytes()
	}
	id := chat.ID(f.users[0].ID)
	request(id, "", 401)
	fallback := request(id, f.tokens[0], 200)
	request(chat.ID(f.users[3].ID), f.tokens[0], 404)
	photo, err := avatar.InitialsPNG("Загруженное фото", "photo")
	if err != nil {
		t.Fatal(err)
	}
	if err = avatar.Save(context.Background(), f.store, f.users[0].ID, avatar.Image{Data: base64.StdEncoding.EncodeToString(photo), Hash: "photo"}); err != nil {
		t.Fatal(err)
	}
	loaded := request(id, f.tokens[0], 200)
	if !bytes.Equal(photo, loaded) || bytes.Equal(fallback, loaded) {
		t.Fatal("uploaded avatar was replaced by initials")
	}
	if err = avatar.Save(context.Background(), f.store, f.users[0].ID, avatar.Image{Data: "invalid base64"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fallback, request(id, f.tokens[0], 200)) {
		t.Fatal("corrupt avatar has no fallback")
	}
}
