package metrics

import (
	"os"
	"strconv"
	"strings"
)

// NetworkSnapshot reports Linux interface counters. Missing counters are not zeros.
func NetworkSnapshot() map[string]any {
	raw, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return map[string]any{"available": false}
	}
	interfaces := []map[string]any{}
	for _, line := range strings.Split(string(raw), "\n") {
		name, counters, ok := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "lo" {
			continue
		}
		fields := strings.Fields(counters)
		if len(fields) < 16 {
			continue
		}
		rx, e1 := strconv.ParseUint(fields[0], 10, 64)
		tx, e2 := strconv.ParseUint(fields[8], 10, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		interfaces = append(interfaces, map[string]any{"name": name, "rx_bytes": rx, "tx_bytes": tx})
	}
	return map[string]any{"available": true, "interfaces": interfaces}
}
