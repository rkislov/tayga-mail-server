package mailstore

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestOpenStreamingMessageAndColdBlob(t *testing.T) {
	for _, folder := range []string{"INBOX", "Archive"} {
		t.Run(folder, func(t *testing.T) {
			s := New(t.TempDir())
			s.SetObjectStore(NewMemBlob(), nil)
			want := "Subject: stream\r\n\r\n" + strings.Repeat("body", 32768)
			path, _, err := s.Deliver("user@example.com", folder, []byte(want))
			if err != nil {
				t.Fatal(err)
			}
			for _, cold := range []bool{false, true} {
				if cold {
					if err := os.Remove(s.Abs(path)); err != nil {
						t.Fatal(err)
					}
				}
				r, err := s.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				prefix := make([]byte, len("Subject: stream"))
				_, err = io.ReadFull(r, prefix)
				if err != nil || string(prefix) != "Subject: stream" {
					t.Fatalf("stream prefix %q: %v", prefix, err)
				}
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
				data, err := s.Read(path)
				if err != nil || string(data) != want {
					t.Fatalf("full message changed: %v", err)
				}
			}
			if folder == "Archive" {
				// Maildir renames can remove the suffix; magic still detects gzip.
				other := s.Abs(path) + ".renamed"
				if err := os.Rename(s.Abs(path), other); err != nil {
					t.Fatal(err)
				}
				data, err := s.Read(other)
				if err != nil || string(data) != want {
					t.Fatalf("gzip magic lost: %v", err)
				}
			}
		})
	}
}
