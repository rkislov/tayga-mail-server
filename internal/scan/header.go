package scan

import (
	"bytes"
	"fmt"
	"strings"
)

// InjectHeader inserts a header just before the blank line separating headers and body.
func InjectHeader(data []byte, name, value string) []byte {
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" {
		return data
	}
	line := fmt.Sprintf("%s: %s\r\n", name, value)

	if idx := bytes.Index(data, []byte("\r\n\r\n")); idx >= 0 {
		var b bytes.Buffer
		b.Grow(len(data) + len(line))
		b.Write(data[:idx+2])
		b.WriteString(line)
		b.Write(data[idx+2:])
		return b.Bytes()
	}
	if idx := bytes.Index(data, []byte("\n\n")); idx >= 0 {
		var b bytes.Buffer
		b.Grow(len(data) + len(line))
		b.Write(data[:idx+1])
		b.WriteString(line)
		b.Write(data[idx+1:])
		return b.Bytes()
	}
	return append([]byte(line), data...)
}
