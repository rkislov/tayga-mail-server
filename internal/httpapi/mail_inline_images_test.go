package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func testMailPhoto(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	photo := image.NewRGBA(image.Rect(0, 0, 8, 8))
	photo.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&b, photo); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}
func TestInlineMailPhotosMIMEAndDelivery(t *testing.T) {
	photo := testMailPhoto(t)
	source := `<h1>Hello</h1><figure class="image" style="width:50%"><img alt="photo" src="` + photo + `"></figure><p>Again <img src="` + photo + `"></p>`
	html, images, err := prepareInlineMailImages(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || strings.Contains(html, "data:") || strings.Count(html, "cid:") != 2 {
		t.Fatal("dedup", html, len(images))
	}
	raw := buildOutgoingMIME("sender@example.com", "Sender", []string{"reader@example.com"}, nil, nil, "Content-Type: Photos", "Hello", html, "<photo@tayga>")
	raw, err = attachInlineMailImages(raw, images)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = attachOutgoingFiles(raw, []outgoingAttachment{{Name: "document.txt", Data: base64.StdEncoding.EncodeToString([]byte("Document"))}})
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseMIMEMessage(raw)
	if len(parsed.Attachments) != 2 || parsed.Attachments[0].ContentID == "" || parsed.Attachments[0].ContentType != "image/png" || parsed.Attachments[1].ContentID != "" || !strings.Contains(parsed.HTML, photo) {
		t.Fatalf("MIME roundtrip: %+v", parsed)
	}
	if !bytes.Contains(raw, []byte("multipart/related")) || !bytes.Contains(raw, []byte("Content-Disposition: inline")) || bytes.Contains(raw, []byte(photo)) {
		t.Fatal("not related MIME")
	}
	f := newFileFixture(t)
	ctx := context.Background()
	user, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	req := map[string]any{"to": []string{"reader@example.com"}, "subject": "Photo mail", "html": source, "text": "Hello"}
	body, _ := json.Marshal(req)
	f.call(0, "POST", "/api/v1/mail/send", string(body), 200)
	reader, err := f.s.store.GetUserByID(ctx, f.users[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []*storage.User{user, reader} {
		folders, err := f.s.store.ListMailboxes(ctx, owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, folder := range folders {
			messages, err := f.s.store.ListMessages(ctx, folder.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range messages {
				content, err := f.s.ms.Read(message.FilePath)
				if err != nil {
					t.Fatal(err)
				}
				parsed := parseMIMEMessage(content)
				if !strings.Contains(parsed.HTML, photo) || len(parsed.Attachments) != 1 || parsed.Attachments[0].ContentID == "" {
					t.Fatal("photo lost on delivery", owner.Email)
				}
				if f.s.hasMailAttachments(message.FilePath) {
					t.Fatal("inline photo shown as a regular attachment")
				}
				found = true
			}
		}
		if !found {
			t.Fatal("missing mail", owner.Email)
		}
	}
	for _, bad := range []string{"data:image/svg+xml;base64,PHN2Zz4=", "data:image/png;base64,AA==", "data:image/jpeg;base64," + strings.TrimPrefix(photo, "data:image/png;base64,")} {
		req["html"] = `<p>Hello<img src="` + bad + `"></p>`
		body, _ = json.Marshal(req)
		f.call(0, "POST", "/api/v1/mail/send", string(body), 400)
	}
}
func TestWelcomePhotosSurviveVersionsRestoreAndValidation(t *testing.T) {
	photo := testMailPhoto(t)
	content := storage.WelcomeRevision{Subject: "Photos", HTML: `<figure class="image image_resized" style="width:50%;background:url(javascript:bad)"><img src="` + photo + `" alt="Our team"><figcaption>Our team</figcaption></figure>`}
	if err := normalizeWelcomeContent(&content); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content.HTML, photo) || !strings.Contains(content.HTML, `style="width:50%"`) || strings.Contains(content.HTML, "background") || content.Text != "Our team" {
		t.Fatal("image lost or unsafe markup", content)
	}
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserRoles(ctx, u.ID, "tenant_admin"); err != nil {
		t.Fatal(err)
	}
	endpoint := "/api/v1/admin/welcome/domains/" + u.DomainID
	body, _ := json.Marshal(storage.WelcomeChange{Action: "save", Content: content})
	f.call(0, "POST", endpoint, string(body), 200)
	f.call(0, "POST", endpoint, `{"action":"restore","expected":1,"revision":1}`, 200)
	saved, err := f.s.store.GetWelcomeTemplate(ctx, "domains", u.DomainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Revisions) != 2 || saved.Revisions[1].HTML != content.HTML || saved.Revisions[1].RestoredFrom != 1 {
		t.Fatal("photo history lost")
	}
	invalid := content
	invalid.HTML = `<p>Photo <img src="data:image/svg+xml;base64,PHN2Zz4="></p>`
	if normalizeWelcomeContent(&invalid) == nil {
		t.Fatal("accepted SVG upload")
	}
	invalid = content
	invalid.HTML = strings.Repeat(`<img src="`+photo+`">`, 21)
	if normalizeWelcomeContent(&invalid) == nil {
		t.Fatal("accepted excessive image count")
	}
}
