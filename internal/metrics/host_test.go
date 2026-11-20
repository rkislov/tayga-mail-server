package metrics

import "testing"

func TestHostCounters(t *testing.T) {
	v := parseHostCounters("cpu  100 20 30 800 40 5 5 0 10 2\ncpu0 10 1 3 80", "MemTotal: 1000 kB\nMemAvailable: 400 kB\nSwapTotal: 200 kB\nSwapFree: 150 kB\n")
	if v["host.cpu_total"] != 1000 || v["host.cpu_idle"] != 840 {
		t.Fatalf("CPU double-counted guest times: %v", v)
	}
	if v["host.memory_used"] != 600*1024 || v["host.swap_used"] != 50*1024 {
		t.Fatalf("memory: %v", v)
	}
	if len(parseHostCounters("cpu bad 1 2 3", "MemTotal: 1000 kB\nMemAvailable: 2000 kB")) != 0 {
		t.Fatal("invalid data must not be zero-filled")
	}
	if len(parseHostCounters("", "")) != 0 {
		t.Fatal("unavailable counters must be omitted")
	}
}
func TestInboundOutcomes(t *testing.T) {
	a, b := mailAccepted.Load(), mailRejected.Load()
	RecordMailEvent("delivered", "inbound")
	RecordMailEvent("rejected", "inbound")
	RecordMailEvent("sent", "outbound")
	RecordMailEvent("smtp_dialogue", "inbound")
	RecordMailEvent("delivered", "outbound")
	if mailAccepted.Load() != a+1 || mailRejected.Load() != b+1 {
		t.Fatal("non-outcome events counted")
	}
}
