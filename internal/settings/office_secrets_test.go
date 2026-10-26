package settings

import (
	"github.com/tayga/tms/internal/config"
	"testing"
)

func TestOfficeSecretRedactionAndPreservation(t *testing.T) {
	old := config.Default()
	old.Files.OfficeJWTSecret = "test-office-secret-at-least-32-bytes"
	neu := *old
	redactConfig(&neu)
	if neu.Files.OfficeJWTSecret != redacted {
		t.Fatal("office JWT secret exposed")
	}
	preserveSecrets(old, &neu)
	if neu.Files.OfficeJWTSecret != old.Files.OfficeJWTSecret {
		t.Fatal("office JWT secret lost")
	}
	for _, field := range ConfigSchema()["files"].Fields {
		if field.Key == "office_jwt_secret" && !field.Secret {
			t.Fatal("office JWT secret visible in form")
		}
	}
}
