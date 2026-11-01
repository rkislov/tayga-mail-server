//go:build !linux && !darwin

package full

import "errors"

const publishSupported = false

func publishDirectory(from, to string) error {
	return errors.New("atomic directory publication is unsupported on this OS")
}
