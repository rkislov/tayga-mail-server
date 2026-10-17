package storage

import "errors"

var ErrCollectedDuplicate = errors.New("source message already collected")
