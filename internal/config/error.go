package config

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrInitConfigCode = "test_code"
)

func ErrInitConfig(err error) error {
	return errors.NewDefault(ErrInitConfigCode, "Error while config init", err.Error())
}
