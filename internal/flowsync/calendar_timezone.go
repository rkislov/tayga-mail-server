// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf16"
)

func calendarZone(id string) (string, error) {
	if id == "" {
		return "UTC", nil
	}
	if zone := windowsZones[id]; zone != "" {
		id = zone
	}
	if id == "Etc/UTC" || id == "Etc/GMT" {
		id = "UTC"
	}
	if id == "Local" {
		return "", fmt.Errorf("local timezone is ambiguous")
	}
	_, err := time.LoadLocation(id)
	return id, err
}

func putZoneName(dst []byte, name string) {
	chars := utf16.Encode([]rune(name))
	if len(chars) > 31 {
		chars = chars[:31]
	}
	for i, c := range chars {
		binary.LittleEndian.PutUint16(dst[i*2:], c)
	}
}
func zoneName(src []byte) string {
	var chars []uint16
	for i := 0; i < len(src); i += 2 {
		v := binary.LittleEndian.Uint16(src[i:])
		if v == 0 {
			break
		}
		chars = append(chars, v)
	}
	return string(utf16.Decode(chars))
}

// MS-ASCAL Timezone: 172-byte TIME_ZONE_INFORMATION, with transitions in
// pre-transition local wall time. Names are labels, never trusted over offsets.
func calendarTZI(zone string, year int) ([]byte, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 172)
	putZoneName(out[4:68], zone)
	putZoneName(out[88:152], zone)
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)
	standard, daylight := 0, 0
	haveStandard, haveDaylight := false, false
	transitions := 0
	for at := start; at.Before(end); {
		local := at.In(loc)
		_, offset := local.Zone()
		if offset%60 != 0 {
			return nil, fmt.Errorf("timezone has sub-minute offsets")
		}
		if local.IsDST() {
			daylight = offset
			haveDaylight = true
		} else {
			standard = offset
			haveStandard = true
		}
		_, next := local.ZoneBounds()
		if next.IsZero() || !next.Before(end) {
			break
		}
		after := next.In(loc)
		_, newOffset := after.Zone()
		if newOffset != offset {
			transitions++
			if transitions > 2 || local.IsDST() == after.IsDST() {
				return nil, fmt.Errorf("timezone cannot be represented by annual ActiveSync rules")
			}
			wall := next.In(time.FixedZone("before", offset))
			date := out[152:168]
			if !after.IsDST() {
				date = out[68:84]
			}
			week := (wall.Day()-1)/7 + 1
			if wall.AddDate(0, 0, 7).Month() != wall.Month() {
				week = 5
			}
			for i, v := range []int{0, int(wall.Month()), int(wall.Weekday()), week, wall.Hour(), wall.Minute(), wall.Second(), 0} {
				binary.LittleEndian.PutUint16(date[i*2:], uint16(v))
			}
		}
		at = next
	}
	if !haveStandard {
		return nil, fmt.Errorf("timezone has no standard offset")
	}
	binary.LittleEndian.PutUint32(out, uint32(int32(-standard/60)))
	if haveDaylight {
		if transitions != 2 {
			return nil, fmt.Errorf("incomplete annual timezone transitions")
		}
		binary.LittleEndian.PutUint32(out[168:], uint32(int32(-(daylight-standard)/60)))
	}
	return out, nil
}
func tziRules(data []byte) []byte {
	out := append([]byte{}, data[:4]...)
	out = append(out, data[68:88]...)
	return append(out, data[152:172]...)
}

func decodeCalendarTimezone(value, preferred string, year int) (string, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(data) != 172 {
		return "", fmt.Errorf("invalid calendar timezone")
	}
	candidates := []string{preferred, zoneName(data[4:68]), zoneName(data[88:152]), "UTC"}
	var rest []string
	for _, zone := range windowsZones {
		rest = append(rest, zone)
	}
	sort.Strings(rest)
	candidates = append(candidates, rest...)
	seen := map[string]bool{}
	for _, id := range candidates {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		zone, e := calendarZone(id)
		if e != nil {
			continue
		}
		expected, e := calendarTZI(zone, year)
		if e == nil && bytes.Equal(tziRules(data), tziRules(expected)) {
			return zone, nil
		}
	}
	return "", fmt.Errorf("calendar timezone is not representable by an IANA zone")
}
