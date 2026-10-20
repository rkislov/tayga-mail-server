package mailsearch

import (
	"bytes"
	"fmt"
	"mime"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/testutil/mailfixture"
)

func TestLegacyMIMEIndex(t *testing.T) {
	for _, cs := range mailfixture.Charsets {
		for _, transfer := range []string{"base64", "quoted-printable", "8bit"} {
			for _, nested := range []bool{false, true} {
				raw := mailfixture.Message(t, cs.Label, transfer, nested)
				before := bytes.Clone(raw)
				doc := ParseDocument(raw)
				if doc.Subject != mailfixture.Subject || doc.From != mailfixture.Sender+" <sender@example.test>" || doc.Body != strings.ToLower(mailfixture.Text) {
					t.Errorf("%s/%s/nested=%v: %+v", cs.Label, transfer, nested, doc)
				}
				if !bytes.Equal(raw, before) {
					t.Fatal("parser mutated source message")
				}
			}
		}
		for _, enc := range []mime.WordEncoder{mime.BEncoding, mime.QEncoding} {
			word := enc.Encode(cs.Label, string(mailfixture.Encode(t, cs.Label, mailfixture.Subject)))
			if got := DecodeHeader(word); got != mailfixture.Subject {
				t.Errorf("%s header: %q", cs.Label, got)
			}
		}
	}
}

func TestIndexHTMLFallbackAndAttachmentIsolation(t *testing.T) {
	raw := "Content-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\nContent-Disposition: attachment; filename=private.txt\r\n\r\nAttachment secret\r\n--b\r\n" +
		"Content-Type: text/html; charset=win-1251\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"<head><style>hidden</style></head><p>=CF=F0=E8=E2=E5=F2 &amp; &#1084;&#1080;&#1088;</p>\r\n--b--\r\n"
	if got := ParseDocument([]byte(raw)).Body; got != "привет & мир" {
		t.Fatalf("HTML fallback: %q", got)
	}
}

func TestIndexBoundsAndUnknownCharset(t *testing.T) {
	if got := ParseDocument([]byte("Content-Type: text/plain; charset=x-unknown\r\n\r\nPreserved ASCII")).Body; got != "preserved ascii" {
		t.Fatalf("unknown charset lost body: %q", got)
	}
	raw := []byte("Content-Type: text/plain; charset=utf-8\r\n\r\n" + strings.Repeat("a", 3<<20))
	if len(ParseDocument(raw).Body) != 2<<20 {
		t.Fatal("text limit not enforced")
	}
	var nested strings.Builder
	for i := 0; i < 22; i++ {
		fmt.Fprintf(&nested, "Content-Type: multipart/mixed; boundary=b%d\r\n\r\n--b%d\r\n", i, i)
	}
	nested.WriteString("Content-Type: text/plain\r\n\r\nToo deep")
	if ParseDocument([]byte(nested.String())).Body != "" {
		t.Fatal("MIME depth limit not enforced")
	}
}
