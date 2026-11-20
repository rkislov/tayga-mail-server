package mailcompose

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	xhtml "golang.org/x/net/html"
)

const MaxEmbeddedImageBytes = 2 << 20
const MaxEmbeddedImagesBytes = 8 << 20
const MaxWelcomeHTMLBytes = 12 << 20

type InlineImage struct {
	ID, Media, Name string
	Data            []byte
}

func DecodeEmbeddedImage(value string) (InlineImage, error) {
	prefix, encoded, ok := strings.Cut(value, ",")
	media := strings.TrimPrefix(prefix, "data:")
	media, base64Encoding := strings.CutSuffix(media, ";base64")
	if !ok || !base64Encoding || (media != "image/png" && media != "image/jpeg" && media != "image/gif") || len(encoded) > base64.StdEncoding.EncodedLen(MaxEmbeddedImageBytes) {
		return InlineImage{}, fmt.Errorf("use PNG, JPEG or GIF images up to 2 MiB")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) > MaxEmbeddedImageBytes {
		return InlineImage{}, fmt.Errorf("invalid embedded image")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	actual := "image/" + format
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 || actual != media {
		return InlineImage{}, fmt.Errorf("invalid image format or dimensions (maximum 4096 × 4096)")
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return InlineImage{}, fmt.Errorf("invalid image data")
	}
	ext := format
	if ext == "jpeg" {
		ext = "jpg"
	}
	digest := sha256.Sum256(data)
	return InlineImage{ID: fmt.Sprintf("%x@tayga-inline", digest), Media: media, Name: "photo." + ext, Data: data}, nil
}

// Preserve original markup outside modified IMG tags, including quoted messages.
// Images become related MIME parts rather than private Files URLs or data URLs.
func PrepareInlineImages(source string) (string, []InlineImage, error) {
	z := xhtml.NewTokenizer(strings.NewReader(source))
	var out strings.Builder
	images := []InlineImage{}
	seen := map[string]bool{}
	total, count := 0, 0
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			if z.Err() != io.EOF {
				return "", nil, z.Err()
			}
			break
		}
		raw := string(z.Raw())
		if tt == xhtml.StartTagToken || tt == xhtml.SelfClosingTagToken {
			t := z.Token()
			if t.Data == "img" {
				changed := false
				for i, a := range t.Attr {
					if a.Key != "src" || !strings.HasPrefix(strings.ToLower(a.Val), "data:") {
						continue
					}
					image, err := DecodeEmbeddedImage(a.Val)
					if err != nil {
						return "", nil, err
					}
					count++
					if count > 20 {
						return "", nil, fmt.Errorf("too many embedded images (maximum 20)")
					}
					if !seen[image.ID] {
						total += len(image.Data)
						if total > MaxEmbeddedImagesBytes {
							return "", nil, fmt.Errorf("embedded images exceed 8 MiB")
						}
						seen[image.ID] = true
						images = append(images, image)
					}
					t.Attr[i].Val = "cid:" + image.ID
					changed = true
				}
				if changed {
					raw = t.String()
				}
			}
		}
		out.WriteString(raw)
	}
	return out.String(), images, nil
}

func AttachInlineImages(raw []byte, images []InlineImage) ([]byte, error) {
	if len(images) == 0 {
		return raw, nil
	}
	split := bytes.Index(raw, []byte("\r\nContent-Type:"))
	if split < 0 {
		return nil, fmt.Errorf("invalid MIME message")
	}
	split += 2 // Start of the header line, never a match inside Subject or From.
	part := raw[split:]
	sep := bytes.Index(part, []byte("\r\n\r\n"))
	if sep < 0 {
		return nil, fmt.Errorf("invalid MIME body")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", strings.TrimSpace(string(part[len("Content-Type:"):sep])))
	p, err := mw.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err = p.Write(part[sep+4:]); err != nil {
		return nil, err
	}
	for _, image := range images {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", image.Media)
		h.Set("Content-ID", "<"+image.ID+">")
		h.Set("Content-Disposition", `inline; filename="`+image.Name+`"`)
		h.Set("Content-Transfer-Encoding", "base64")
		p, err = mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(image.Data)
		for len(encoded) > 76 {
			fmt.Fprintf(p, "%s\r\n", encoded[:76])
			encoded = encoded[76:]
		}
		fmt.Fprintf(p, "%s\r\n", encoded)
	}
	if err = mw.Close(); err != nil {
		return nil, err
	}
	out := append([]byte{}, raw[:split]...)
	out = append(out, []byte("Content-Type: multipart/related; boundary="+mw.Boundary()+"\r\n\r\n")...)
	return append(out, body.Bytes()...), nil
}
