package httpapi

import (
	"testing"

	"github.com/tayga/tms/internal/storage"
)

// Invalid identities must fail before touching either the store or filesystem.
func TestFileRootsRejectPathIdentities(t *testing.T) {
	s := &Server{}
	for _, owner := range []string{"", ".", "..", "../outside", "a/b", `a\b`, storage.NewID() + "/../outside", storage.NewID() + "\x00"} {
		t.Run(owner, func(t *testing.T) {
			if _, err := s.filesRoot(owner); err != errPathEscape {
				t.Fatalf("filesRoot: %v", err)
			}
			if _, err := s.fileStateRoot(owner, "versions"); err != errPathEscape {
				t.Fatalf("fileStateRoot: %v", err)
			}
			if _, err := s.listSpaceCheckpoints(owner); err != errPathEscape {
				t.Fatalf("listSpaceCheckpoints: %v", err)
			}
		})
	}
	for _, kind := range []string{"", "..", "../outside", "versions/../uploads", "unknown"} {
		if _, err := s.fileStateRoot(storage.NewID(), kind); err != errPathEscape {
			t.Fatalf("kind %q: %v", kind, err)
		}
	}
}
