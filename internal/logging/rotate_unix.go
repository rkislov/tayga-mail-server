//go:build !windows

package logging

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func watchRotation(file *rotatingFile) func() {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	exited := make(chan struct{})
	signal.Notify(signals, syscall.SIGHUP)
	go func() {
		defer close(exited)
		for {
			select {
			case <-signals:
				if err := file.reopen(); err != nil {
					fmt.Fprintf(os.Stderr, "tayga: reopen log: %v\n", err)
				}
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { signal.Stop(signals); close(done); <-exited }) }
}
