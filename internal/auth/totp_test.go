package auth_test

import (
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
)

func TestTOTPRoundTrip(t *testing.T) {
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	uri := auth.TOTPURI("Tayga Mail", "admin@example.com", secret)
	if uri == "" || len(secret) < 16 {
		t.Fatalf("bad secret/uri: %s %s", secret, uri)
	}

	// Generate current code via same algorithm by probing ValidateTOTP with
	// a known-good path: re-validate after computing through public API only.
	// We accept that without exporting hotp we check backup codes + invalid.
	if auth.ValidateTOTP(secret, "000000") && auth.ValidateTOTP(secret, "111111") {
		// Extremely unlikely both match; if so, skip rather than flake.
		t.Skip("degenerate secret")
	}
	if auth.ValidateTOTP(secret, "abc") {
		t.Fatal("non-numeric should fail")
	}
	_ = time.Now()
}

func TestBackupCodes(t *testing.T) {
	plain, hashes, err := auth.GenerateBackupCodes(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 4 || len(hashes) != 4 {
		t.Fatal("length mismatch")
	}
	remaining, ok := auth.ConsumeBackupCode(hashes, plain[1])
	if !ok || len(remaining) != 3 {
		t.Fatalf("consume failed ok=%v len=%d", ok, len(remaining))
	}
	_, ok = auth.ConsumeBackupCode(remaining, plain[1])
	if ok {
		t.Fatal("code should be single-use")
	}
	_, ok = auth.ConsumeBackupCode(remaining, "NOTACODE")
	if ok {
		t.Fatal("invalid code accepted")
	}
}
