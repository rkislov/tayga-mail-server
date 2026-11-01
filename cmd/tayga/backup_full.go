package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"filippo.io/age"
	"github.com/tayga/tms/internal/backup/full"
)

func runFullBackup(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	switch command {
	case "backup-keygen":
		out := fs.String("out", "", "new private age identity file; keep off the server")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *out == "" {
			return errors.New("-out is required")
		}
		key, err := age.GenerateX25519Identity()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(f, key.String())
		err = errors.Join(writeErr, f.Sync(), f.Close())
		if err != nil {
			os.Remove(*out)
			return err
		}
		fmt.Printf("public recipient: %s\n", key.Recipient())
		return nil
	case "backup-full":
		root := fs.String("root", "", "self-contained installation root")
		cfg := fs.String("config", "", "absolute YAML config path within root")
		out := fs.String("out", "", "new encrypted .tar.gz.age file outside root")
		recipient := fs.String("recipient", "", "public age1... recipient")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *root == "" || *cfg == "" || *out == "" || *recipient == "" {
			return errors.New("-root, -config, -out and -recipient are required")
		}
		m, err := full.Snapshot(ctx, full.Options{Root: *root, Config: *cfg, Output: *out, Recipient: *recipient})
		if err != nil {
			return err
		}
		fmt.Printf("encrypted snapshot created: %s; entries=%d tables=%d; run verify-full with the private identity on an isolated machine\n", *out, len(m.Entries), len(m.Counts))
		return nil
	case "restore-full", "verify-full":
		in := fs.String("in", "", "encrypted snapshot file")
		identity := fs.String("identity", "", "private age identity file")
		target := fs.String("target", "", "new installation directory (must not exist)")
		scratch := fs.String("scratch", os.TempDir(), "parent directory for isolated verification")
		max := fs.Int64("max-bytes", 100<<30, "maximum restored file bytes")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *in == "" || *identity == "" || *max <= 0 || *max > 1<<60 {
			return errors.New("-in, -identity and a positive bounded -max-bytes are required")
		}
		verify := command == "verify-full"
		if verify {
			dir, err := os.MkdirTemp(*scratch, "tayga-verify-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			*target = filepath.Join(dir, "installation")
		} else if *target == "" {
			return errors.New("-target is required")
		}
		m, err := full.Restore(ctx, full.RestoreOptions{Input: *in, Identity: *identity, Target: *target, MaxBytes: *max, VerifyOnly: verify})
		if err != nil {
			return err
		}
		if verify {
			fmt.Printf("verified: authenticated encryption, %d entries, SQLite integrity and %d table counts, referenced content and server key paths\n", len(m.Entries), len(m.Counts))
		} else {
			fmt.Printf("restored and verified: %s; config: %s; service was not started\n", *target, filepath.Join(*target, m.Header.Config))
		}
		return nil
	}
	return errors.New("unknown full backup command")
}
