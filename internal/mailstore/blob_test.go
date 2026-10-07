package mailstore

import (
	"os"
	"testing"
)

func TestStoreObjectStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ms := New(dir)
	blob := NewMemBlob()
	ms.SetObjectStore(blob, nil)

	payload := []byte("From: a\r\n\r\nhello")
	rel, size, err := ms.Deliver("u@ex.com", "INBOX", payload)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(payload)) {
		t.Fatalf("size %d", size)
	}
	key := blobKey(rel)
	if _, ok := blob.m[key]; !ok {
		t.Fatalf("missing blob key %q", key)
	}

	// Cache miss: remove local file, Read should refill from blob.
	if err := os.Remove(ms.Abs(rel)); err != nil {
		t.Fatal(err)
	}
	data, err := ms.Read(rel)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "From: a\r\n\r\nhello" {
		t.Fatalf("got %q", data)
	}
	if _, err := os.Stat(ms.Abs(rel)); err != nil {
		t.Fatal("expected cache repopulated")
	}

	newRel, err := ms.MoveToCur(rel, []string{`\Seen`})
	if err != nil {
		t.Fatal(err)
	}
	if newRel == rel {
		t.Fatal("expected path change to cur/")
	}
	if _, ok := blob.m[blobKey(rel)]; ok {
		t.Fatal("old key should be deleted")
	}
	if _, ok := blob.m[blobKey(newRel)]; !ok {
		t.Fatal("new key missing")
	}

	if err := ms.Delete(newRel); err != nil {
		t.Fatal(err)
	}
	if _, ok := blob.m[blobKey(newRel)]; ok {
		t.Fatal("delete should remove blob")
	}
}
