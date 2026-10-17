package settings

import (
	"github.com/tayga/tms/internal/config"
	"testing"
)

func TestLearningControllerPasswordRedactionAndPreservation(t *testing.T) {
	old := config.Default()
	old.Spam.Learning.Password = "controller-fixture-secret"
	neu := *old
	redactConfig(&neu)
	if neu.Spam.Learning.Password != "***" {
		t.Fatal("controller password exposed")
	}
	preserveSecrets(old, &neu)
	if neu.Spam.Learning.Password != old.Spam.Learning.Password {
		t.Fatal("controller password lost")
	}
}
