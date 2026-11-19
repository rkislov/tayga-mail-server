package mmplugins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/avatar"
	"github.com/tayga/tms/internal/storage"
)

func (a *hostAPI) SetProfileImage(userID string, data []byte) *model.AppError {
	u, err := a.user(userID)
	if err != nil {
		return appError(err)
	}
	// Plugins may change only their own scoped bot, never a human or another
	// plugin's bot. Reuse TMS avatar storage so native clients see the image too.
	if _, e := a.GetBot(userID, false); e != nil {
		return e
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return appError(ErrInvalid)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return appError(ErrInvalid)
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return appError(ErrInvalid)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, decoded); err != nil {
		return appError(err)
	}
	if out.Len() > 1<<20 {
		return appError(ErrInvalid)
	}
	digest := sha256.Sum256(out.Bytes())
	return appError(avatar.Save(context.Background(), a.runtime.options.Store, u.ID, avatar.Image{Data: base64.StdEncoding.EncodeToString(out.Bytes()), Hash: hex.EncodeToString(digest[:])}))
}

func (a *hostAPI) GetProfileImage(userID string) ([]byte, *model.AppError) {
	u, err := a.user(userID)
	if err != nil {
		return nil, appError(err)
	}
	value, err := avatar.Get(context.Background(), a.runtime.options.Store, u.ID)
	if err != nil {
		return nil, appError(err)
	}
	if value.Data == "" {
		return nil, appError(storage.ErrNotFound)
	}
	data, err := base64.StdEncoding.DecodeString(value.Data)
	return data, appError(err)
}
