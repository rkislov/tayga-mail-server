package flowsync

import (
	"bytes"
	"strings"
	"testing"
)

func TestProtocolTraceRedactsContent(t *testing.T) {
	x := `<Envelope><Password>password-secret</Password><Body><Status>1</Status><Subject>subject-secret</Subject><Data>body-secret</Data><FolderId Id="id-secret"/></Body></Envelope>`
	result := strings.Join(protocolStructure([]byte(x)), " ")
	for _, secret := range []string{"password-secret", "subject-secret", "body-secret", "id-secret"} {
		if strings.Contains(result, secret) {
			t.Fatal("secret leaked", secret)
		}
	}
	if !strings.Contains(result, "<Status> 1 </Status>") {
		t.Fatal(result)
	}
	e := newWBEncoder()
	e.start(tagFHFolderSync)
	e.taggedStr(tagFHStatus, "1")
	e.taggedStr(tagFHDisplayName, "folder-secret")
	e.end()
	result = strings.Join(protocolStructure(e.bytes()), " ")
	if strings.Contains(result, "folder-secret") || !strings.Contains(result, "[redacted]") {
		t.Fatal(result)
	}
	var b traceBuffer
	p := bytes.Repeat([]byte("x"), traceLimit+100)
	n, err := b.Write(p)
	if err != nil || n != len(p) || len(b.data) != traceLimit || !b.truncated {
		t.Fatal("capture limit broken")
	}
	for _, bad := range [][]byte{{3}, {3, 1, 106, 255}, {3, 1, 106, 0, 0}, {3, 1, 106, 0, 195, 255}} {
		protocolStructure(bad)
	}
}
