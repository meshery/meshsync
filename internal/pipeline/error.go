package pipeline

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrListCode          = "1001"
	ErrPublishCode       = "1002"
	ErrDynamicClientCode = "1003"
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
