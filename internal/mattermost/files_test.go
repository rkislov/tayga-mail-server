package mattermost

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
)

func TestUploadAttachAndDownloadFromPrivateChannel(t *testing.T) {
	f := setup(t)
	if err := f.api.SetAttachmentDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	channel := f.call(t, "POST", "/channels/direct", f.tokens[0], []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)["id"].(string)
	config := f.call(t, "GET", "/config/client?format=old", "", nil, 200)
	if config["EnableMobileFileUpload"] != "true" || config["EnableFileAttachments"] != "true" {
		t.Fatal("attachment support is not advertised")
	}

	request := func(method, path, token, contentType string, body io.Reader, expected int, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/v4"+path, body)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("Content-Type", contentType)
		for key, val := range headers {
			r.Header.Set(key, val)
		}
		w := httptest.NewRecorder()
		f.api.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("%s %s: got %d, expected %d: %s", method, path, w.Code, expected, w.Body.String())
		}
		return w
	}

	payload := []byte("specific bytes for a private download")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "образец.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = writer.WriteField("channel_id", channel); err != nil {
		t.Fatal(err)
	}
	if err = writer.WriteField("client_ids", "local-1"); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	raw := request(http.MethodPost, "/files", f.tokens[0], writer.FormDataContentType(), &body, 201, nil)
	var uploaded struct {
		FileInfos []struct{ ID, Name string } `json:"file_infos"`
		Clients   []string                    `json:"client_ids"`
	}
	if err = json.Unmarshal(raw.Body.Bytes(), &uploaded); err != nil || len(uploaded.FileInfos) != 1 ||
		uploaded.FileInfos[0].Name != "образец.txt" || len(uploaded.Clients) != 1 || uploaded.Clients[0] != "local-1" {
		t.Fatalf("invalid upload response: %v, %+v", err, uploaded)
	}
	id := uploaded.FileInfos[0].ID
	filePath := "/files/" + id
	f.call(t, "GET", filePath+"/info", f.tokens[0], nil, 200)
	f.call(t, "GET", filePath+"/info", f.tokens[1], nil, 404)
	f.call(t, "POST", "/posts", f.tokens[1], map[string]any{"channel_id": channel, "file_ids": []string{id}}, 400)
	f.call(t, "POST", "/posts", f.tokens[0], map[string]any{"channel_id": channel, "file_ids": []string{id, id}}, 400)
	input := map[string]any{"channel_id": channel, "file_ids": []string{id}, "pending_post_id": "test-attach-first"}
	post := f.call(t, "POST", "/posts", f.tokens[0], input, 201)
	if retry := f.call(t, "POST", "/posts", f.tokens[0], input, 201); retry["id"] != post["id"] {
		t.Fatal("duplicate post")
	}
	if list, ok := post["file_ids"].([]any); !ok || len(list) != 1 || list[0] != id {
		t.Fatal("missing file ID")
	}
	postPath := "/posts/" + post["id"].(string)
	fetched := f.call(t, "GET", postPath, f.tokens[1], nil, 200)
	if files, ok := fetched["metadata"].(map[string]any)["files"].([]any); !ok || len(files) != 1 {
		t.Fatal("missing persisted attachment metadata")
	}
	request(http.MethodGet, postPath+"/files", f.tokens[1], "", nil, 200, nil)
	for _, index := range []int{2, 3} {
		request(http.MethodGet, filePath, f.tokens[index], "", nil, 404, nil)
	}
	request(http.MethodGet, filePath, "", "", nil, 401, nil)
	downloaded := request(http.MethodGet, filePath, f.tokens[1], "", nil, 200, nil)
	if !bytes.Equal(downloaded.Body.Bytes(), payload) || !strings.HasPrefix(downloaded.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("download bytes or headers differ")
	}
	partial := request(http.MethodGet, filePath, f.tokens[1], "", nil, 206, map[string]string{"Range": "bytes=0-7"})
	if partial.Body.String() != string(payload[:8]) {
		t.Fatal("range download mismatch")
	}
	// Rows from the previous release keep their original BLOB contents.
	if _, err := f.store.DB().Exec("UPDATE chat_files SET on_disk=0,content=? WHERE id=?", payload, id); err != nil {
		t.Fatal(err)
	}
	legacy := request(http.MethodGet, filePath, f.tokens[1], "", nil, 200, nil)
	if !bytes.Equal(legacy.Body.Bytes(), payload) {
		t.Fatal("legacy attachment unavailable after migration")
	}

	uploadPath := "/files?channel_id=" + url.QueryEscape(channel) + "&filename=second.txt"
	request(http.MethodPost, uploadPath, f.tokens[1], "application/octet-stream", strings.NewReader("raw upload"), 201, nil)
	request(http.MethodPost, uploadPath, f.tokens[2], "application/octet-stream", strings.NewReader("private"), 404, nil)
	oversized := httptest.NewRequest(http.MethodPost, "/api/v4"+uploadPath, strings.NewReader("unused"))
	oversized.Header.Set("Authorization", "Bearer "+f.tokens[1])
	oversized.ContentLength = chat.MaxAttachmentBytes + 1
	ignored := httptest.NewRecorder()
	f.api.ServeHTTP(ignored, oversized)
	if ignored.Code != 413 {
		t.Fatalf("oversized upload accepted: %d", ignored.Code)
	}

	const width, height = 4284, 5712 // Orientation 6 rotates the stored pixels 90°.
	var photo bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewGray(image.Rect(0, 0, height, width)), nil); err != nil {
		t.Fatal(err)
	}
	exif, err := hex.DecodeString("ffe1002245786966000049492a0008000000010012010300010000000600000000000000")
	if err != nil {
		t.Fatal(err)
	}
	photoBytes := bytes.Join([][]byte{photo.Bytes()[:2], exif, photo.Bytes()[2:]}, nil)
	response := request(http.MethodPost, "/files?channel_id="+channel+"&filename=photo.jpg",
		f.tokens[0], "image/jpeg", bytes.NewReader(photoBytes), 201, nil)
	var imageUpload struct {
		Files []struct {
			ID     string
			Width  int
			Height int
		} `json:"file_infos"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &imageUpload); err != nil || len(imageUpload.Files) != 1 ||
		imageUpload.Files[0].Width != width || imageUpload.Files[0].Height != height {
		t.Fatalf("invalid photo metadata: %+v (%v)", imageUpload, err)
	}
	photoID := imageUpload.Files[0].ID
	photoPost := f.call(t, "POST", "/posts", f.tokens[0], map[string]any{"channel_id": channel, "file_ids": []string{photoID}}, 201)
	photoPostID := photoPost["id"].(string)
	if _, err := f.store.DB().Exec("UPDATE chat_files SET width=0,height=0 WHERE id=?", photoID); err != nil {
		t.Fatal(err)
	}
	if err := f.api.chat.RepairAttachmentImageDimensions(context.Background()); err != nil {
		t.Fatal(err)
	}
	photoPath := "/files/" + photoID
	meta := f.call(t, "GET", photoPath+"/info", f.tokens[1], nil, 200)
	if meta["width"] != float64(width) || meta["height"] != float64(height) || meta["has_preview_image"] != true {
		t.Fatalf("photo metadata was not repaired: %v", meta)
	}
	// An earlier release could have fixed the file row but left its post
	// snapshot with zero dimensions. The gallery reads the snapshot.
	if _, err := f.store.DB().Exec(`UPDATE chat_posts SET attachments=replace(replace(attachments,'"width":4284','"width":0'),'"height":5712','"height":0') WHERE id=?`, photoPostID); err != nil {
		t.Fatal(err)
	}
	if err := f.api.chat.RepairAttachmentImageDimensions(context.Background()); err != nil {
		t.Fatal(err)
	}
	history := f.call(t, "GET", "/channels/"+channel+"/posts", f.tokens[1], nil, 200)
	withPhoto := history["posts"].(map[string]any)[photoPostID].(map[string]any)
	snapshot := withPhoto["metadata"].(map[string]any)["files"].([]any)[0].(map[string]any)
	if snapshot["width"] != float64(width) || snapshot["height"] != float64(height) {
		t.Fatalf("post history still reports stale image dimensions: %v", snapshot)
	}
	preview := request(http.MethodGet, photoPath+"/preview", f.tokens[1], "", nil, 200, nil)
	if preview.Header().Get("Content-Type") != "image/jpeg" || !bytes.Equal(preview.Body.Bytes(), photoBytes) {
		t.Fatal("invalid photo preview content")
	}
	request(http.MethodGet, photoPath+"/preview", f.tokens[2], "", nil, 404, nil)
}
