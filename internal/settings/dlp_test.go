package settings

import (
	"github.com/tayga/tms/internal/config"
	"testing"
)

func TestDLPSecretRedactionAndPreservation(t *testing.T) {
	old := config.Default()
	old.SMTP.DLP.Password = "dlp-secret"
	neu := *old
	redactConfig(&neu)
	if neu.SMTP.DLP.Password != "***" {
		t.Fatal("DLP password exposed")
	}
	preserveSecrets(old, &neu)
	if neu.SMTP.DLP.Password != old.SMTP.DLP.Password {
		t.Fatal("DLP password lost")
	}
}
