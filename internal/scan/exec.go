package scan

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Exec runs an external scanner with the message on stdin.
// Exit 0 = clean; non-zero = infected (or error if stderr looks like a failure and FailOpen handled upstream).
type Exec struct {
	argv    []string
	timeout time.Duration
}

func NewExec(argv []string, timeout time.Duration) *Exec {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cp := append([]string(nil), argv...)
	return &Exec{argv: cp, timeout: timeout}
}

func (e *Exec) Name() string { return "exec" }

func (e *Exec) Scan(ctx context.Context, data []byte) (*Result, error) {
	if len(e.argv) == 0 {
		return nil, fmt.Errorf("%w: empty command", ErrUnavailable)
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.argv[0], e.argv[1:]...)
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := &Result{Scanner: "exec:" + e.argv[0], Duration: time.Since(start)}
	out := strings.TrimSpace(stdout.String() + "\n" + stderr.String())

	if err == nil {
		res.Clean = true
		return res, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: timeout", ErrUnavailable)
	}
	var ee *exec.ExitError
	if ok := asExitError(err, &ee); ok {
		// Treat non-zero as infected (clamdscan, antivirus CLI convention).
		res.Clean = false
		res.Virus = firstLine(out)
		if res.Virus == "" {
			res.Virus = fmt.Sprintf("exit %d", ee.ExitCode())
		}
		return res, nil
	}
	return nil, fmt.Errorf("%w: %v (%s)", ErrUnavailable, err, firstLine(out))
}

func asExitError(err error, dest **exec.ExitError) bool {
	if err == nil {
		return false
	}
	if ee, ok := err.(*exec.ExitError); ok {
		*dest = ee
		return true
	}
	return false
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
