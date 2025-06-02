package pipeline

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrListCode          = "1001"
	ErrPublishCode       = "1002"
	ErrDynamicClientCode = "1003"
	ErrCacheSyncCode     = "1014"
	ErrWriteOutputCode   = "1015"
)

func ErrDynamicClient(name string, err error) error {
	return errors.New(ErrDynamicClientCode, errors.Alert, []string{"Error creating dynamic client for: " + name, err.Error()}, []string{}, []string{}, []string{})
}

func ErrList(name string, err error) error {
	return errors.New(ErrListCode, errors.Alert, []string{"Error while listing: " + name, err.Error()}, []string{}, []string{}, []string{})
}

func ErrPublish(name string, err error) error {
	return errors.New(ErrPublishCode, errors.Alert, []string{"Error while publishing for: " + name, err.Error()}, []string{}, []string{}, []string{})
}

func ErrCacheSync(name string, err error) error {
	return errors.New(ErrCacheSyncCode, errors.Alert, []string{"Error while syncing the informer store for: " + name, err.Error()}, []string{}, []string{}, []string{})
}

func ErrWriteOutput(name string, err error) error {
	return errors.New(ErrWriteOutputCode, errors.Alert, []string{"Error while writing output for: " + name, err.Error()}, []string{}, []string{}, []string{})
}
