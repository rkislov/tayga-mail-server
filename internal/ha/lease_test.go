package ha

import "testing"

func TestDefaultLockKeysStable(t *testing.T) {
	a1, a2 := DefaultLockKeys()
	b1, b2 := DefaultLockKeys()
	if a1 != b1 || a2 != b2 {
		t.Fatal("keys must be stable")
	}
	if a1 == 0 && a2 == 0 {
		t.Fatal("keys should be non-zero")
	}
}

func TestAlwaysLeader(t *testing.T) {
	var g Gate = AlwaysLeader{}
	if !g.IsLeader() {
		t.Fatal("expected leader")
	}
}
