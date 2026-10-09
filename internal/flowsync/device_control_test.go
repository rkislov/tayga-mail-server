// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bytes"
	"testing"
)

func TestAccountOnlyWipeAcknowledgment(t *testing.T) {
	if !accountWipeAcknowledged([]byte(`<Provision xmlns="Provision:"><AccountOnlyRemoteWipe><Status>1</Status></AccountOnlyRemoteWipe></Provision>`), false) {
		t.Fatal("valid XML acknowledgment rejected")
	}
	for _, body := range []string{`<Provision xmlns="Provision:"><Policies><Status>1</Status></Policies></Provision>`, `<Provision xmlns="Provision:"><AccountOnlyRemoteWipe><Status>2</Status></AccountOnlyRemoteWipe></Provision>`} {
		if accountWipeAcknowledged([]byte(body), false) {
			t.Fatal("unrelated or failed acknowledgment accepted")
		}
	}
	e := newWBEncoder()
	e.start(tagProvProvision)
	e.start(wbTag{cpProvision, 0x3b})
	e.taggedStr(tagProvStatus, "1")
	e.end()
	e.end()
	if !accountWipeAcknowledged(e.bytes(), true) {
		t.Fatal("WBXML acknowledgment rejected")
	}
	command := encodeAccountWipeWBXML(false)
	if accountWipeAcknowledged(command, true) {
		t.Fatal("directive mistaken for acknowledgment")
	}
	if bytes.Contains(command, []byte{0x4c}) {
		t.Fatal("full-device remote wipe emitted")
	}
}
