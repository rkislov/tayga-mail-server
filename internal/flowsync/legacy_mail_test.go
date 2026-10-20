package flowsync

import (
	"bytes"
	"testing"

	"github.com/tayga/tms/internal/testutil/mailfixture"
)

func TestLegacyMessageContent(t *testing.T) {
	for _, cs := range mailfixture.Charsets {
		for _, transfer := range []string{"base64", "quoted-printable", "8bit"} {
			t.Run(cs.Label+"/"+transfer, func(t *testing.T) {
				raw := mailfixture.Message(t, cs.Label, transfer, true)
				got, err := parseMessageContent(raw)
				if err != nil {
					t.Fatal(err)
				}
				if got.Subject != mailfixture.Subject || got.Text != mailfixture.Text || got.HTML != mailfixture.HTML || got.From != mailfixture.Sender+" <sender@example.test>" {
					t.Fatalf("decoded content: %+v", got)
				}
				if !bytes.Equal(got.MIME, raw) {
					t.Fatal("original MIME changed")
				}
				if len(got.Attachments) != 1 || got.Attachments[0].Name != mailfixture.Filename || !bytes.Equal(got.Attachments[0].Data, mailfixture.Attachment) {
					t.Fatalf("attachment: %+v", got.Attachments)
				}
			})
		}
	}
}
