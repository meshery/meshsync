package pipeline

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrListCode          = "test_code"
	ErrPublishCode       = "test_code"
	ErrDynamicClientCode = "test_code"
)

func ErrDynamicClient(name string, err error) error {
	return errors.NewDefault(ErrDynamicClientCode, "Error creating dynamic client for: "+name, err.Error())
}

func ErrList(name string, err error) error {
	return errors.NewDefault(ErrListCode, "Error while listing: "+name, err.Error())
}

func ErrPublish(name string, err error) error {
	return errors.NewDefault(ErrPublishCode, "Error while publishing for: "+name, err.Error())
}
