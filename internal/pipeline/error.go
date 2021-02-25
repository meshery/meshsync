package pipeline

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrListCode    = "test_code"
	ErrPublishCode = "test_code"
)

func ErrList(name string, err error) error {
	return errors.NewDefault(ErrListCode, "Error while listing: "+name, err.Error())
}

func ErrPublish(name string, err error) error {
	return errors.NewDefault(ErrPublishCode, "Error while publishing for: "+name, err.Error())
}
