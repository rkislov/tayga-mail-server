package ha_test

import (
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/ha"
)

type stubGate struct{ leader bool }

func (s stubGate) IsLeader() bool { return s.leader }

func TestFenceWritersConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.Driver = "postgres"
	cfg.Storage.Postgres.DSN = "postgres://x"
	cfg.HA.Mode = "active_standby"
	cfg.HA.Fence = "writers"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.HA.FenceWriters() {
		t.Fatal("expected writers fence")
	}
	cfg.HA.Fence = "mx"
	_ = cfg.Validate()
	if cfg.HA.FenceWriters() {
		t.Fatal("mx should not fence writers")
	}
}

func TestStandbyGate(t *testing.T) {
	g := stubGate{leader: false}
	if g.IsLeader() {
		t.Fatal()
	}
	_ = ha.ErrStandby
}
