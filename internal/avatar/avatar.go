package avatar

import (
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
)

type Image struct {
	Data string `json:"data"`
	Hash string `json:"hash"`
}

func Get(ctx context.Context, store storage.Driver, userID string) (Image, error) {
	value, _, err := store.GetSetting(ctx, "profile.avatar."+userID)
	if err != nil {
		return Image{}, err
	}
	if value == "" {
		return Image{}, nil
	}
	var image Image
	err = json.Unmarshal([]byte(value), &image)
	return image, err
}
func Save(ctx context.Context, store storage.Driver, userID string, image Image) error {
	data, err := json.Marshal(image)
	if err != nil {
		return err
	}
	return store.PutSetting(ctx, "profile.avatar."+userID, string(data))
}
