package informer

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrCreateGWatcherCode = "test_code"
	ErrCreateLWatcherCode = "test_code"
)

func ErrCreateGWatcher(name string, err error) error {
	return errors.NewDefault(ErrCreateGWatcherCode, "Error while creating global watcher for: "+name, err.Error())
}

func ErrCreateLWatcher(name string, err error) error {
	return errors.NewDefault(ErrCreateLWatcherCode, "Error while creating local watcher for: "+name, err.Error())
}
