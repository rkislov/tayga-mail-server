package metrics

import "sync"

// Protection counters count per-recipient SMTP decisions since process start.
// They are intentionally separate from the number of stored Junk messages.
var protection = struct {
	sync.Mutex
	decisions   map[string]uint64
	newsletters uint64
}{decisions: make(map[string]uint64)}

func RecordProtection(action string, newsletter bool) {
	protection.Lock()
	defer protection.Unlock()
	protection.decisions[action]++
	if newsletter {
		protection.newsletters++
	}
}

func ProtectionSnapshot() map[string]any {
	protection.Lock()
	defer protection.Unlock()
	decisions := make(map[string]uint64, len(protection.decisions))
	for k, v := range protection.decisions {
		decisions[k] = v
	}
	return map[string]any{"decisions": decisions, "newsletters": protection.newsletters, "scope": "per_recipient_since_process_start"}
}
