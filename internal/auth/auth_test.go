package auth_test

import (
	"testing"

	"github.com/tayga/tms/internal/auth"
)

func TestArgon2idRoundTrip(t *testing.T) {
	h := auth.Argon2id{}
	encoded, err := h.Hash("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := h.Verify(encoded, "secret-password")
	if err != nil || !ok {
		t.Fatalf("verify failed: ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify(encoded, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected mismatch")
	}
}
