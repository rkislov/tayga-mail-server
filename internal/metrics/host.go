package metrics

import (
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

var mailAccepted, mailRejected atomic.Uint64

// RecordMailEvent counts inbound recipient outcomes, independently of logging I/O.
func RecordMailEvent(event, direction string) {
	if direction != "inbound" {
		return
	}
	switch event {
	case "delivered":
		mailAccepted.Add(1)
	case "rejected":
		mailRejected.Add(1)
	}
}

// HostSnapshot uses Linux host counters. Unavailable values are omitted, never zero-filled.
// CPU guest times are already included in user/nice and must not be added again.
func HostSnapshot() map[string]float64 {
	cpu, _ := os.ReadFile("/proc/stat")
	memory, _ := os.ReadFile("/proc/meminfo")
	return parseHostCounters(string(cpu), string(memory))
}
func parseHostCounters(cpu, memory string) map[string]float64 {
	values := map[string]float64{}
	for _, line := range strings.Split(cpu, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var total, idle float64
		valid := true
		for i := 1; i < len(fields) && i <= 8; i++ {
			n, err := strconv.ParseFloat(fields[i], 64)
			if err != nil || n < 0 {
				valid = false
				break
			}
			total += n
			if i == 4 || i == 5 {
				idle += n
			}
		}
		if valid && total > 0 {
			values["host.cpu_total"] = total
			values["host.cpu_idle"] = idle
		}
		break
	}
	mem := map[string]float64{}
	for _, line := range strings.Split(memory, "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || f[2] != "kB" {
			continue
		}
		n, err := strconv.ParseFloat(f[1], 64)
		if err == nil && n >= 0 {
			mem[strings.TrimSuffix(f[0], ":")] = n * 1024
		}
	}
	if total, ok := mem["MemTotal"]; ok && total > 0 {
		if available, ok := mem["MemAvailable"]; ok && available <= total {
			values["host.memory_total"] = total
			values["host.memory_used"] = total - available
		}
	}
	if total, ok := mem["SwapTotal"]; ok {
		if free, ok := mem["SwapFree"]; ok && free <= total {
			values["host.swap_total"] = total
			values["host.swap_used"] = total - free
		}
	}
	return values
}
