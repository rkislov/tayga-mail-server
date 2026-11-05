package flowsync

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestCalendarTimezoneIsWBXMLInlineBase64String(t *testing.T) {
	for _, zone := range []string{"Asia/Yekaterinburg", "America/New_York", "UTC"} {
		t.Run(zone, func(t *testing.T) {
			tzi, err := calendarTZI(zone, 2026)
			if err != nil {
				t.Fatal(err)
			}
			value := base64.StdEncoding.EncodeToString(tzi)
			text := `<Sync xmlns="AirSync:"><Collections><Collection><Commands><Change><ServerId>cached-series</ServerId><ApplicationData><Timezone xmlns="Calendar:">` + value + `</Timezone></ApplicationData></Change></Commands></Collection></Collections></Sync>`
			wire, err := encodeWire(text)
			if err != nil {
				t.Fatal(err)
			}
			// Independent wire assertion: Calendar page, Timezone with content,
			// STR_I, base64 text, NUL, END. Roundtripping our own decoder would
			// also accept the nonconforming OPAQUE encoding used previously.
			want := append([]byte{0, 4, 0x45, 0x03}, []byte(value)...)
			want = append(want, 0, 1)
			if !bytes.Contains(wire, want) {
				t.Fatal("Timezone must use a WBXML inline base64 string")
			}
			decoded, err := decodeWire(wire)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parseProtocolXML(decoded)
			if err != nil || doc.find("Timezone").Text != value {
				t.Fatal("timezone changed", err)
			}
			// Continue accepting old clients' binary form on input. TimeZone is
			// 172 bytes, whose WBXML multi-byte length is 0x81 0x2c.
			legacy := []byte{3, 1, 0x6a, 0, 0, 4, 0x45, 0xc3, 0x81, 0x2c}
			legacy = append(legacy, tzi...)
			legacy = append(legacy, 1)
			decoded, err = decodeWire(legacy)
			if err != nil {
				t.Fatal(err)
			}
			doc, err = parseProtocolXML(decoded)
			if err != nil || doc.find("Timezone").Text != value {
				t.Fatal("legacy input timezone changed", err)
			}
		})
	}
}
