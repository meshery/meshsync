package config

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrInitConfigCode = "1000"
)

func ErrInitConfig(err error) error {
	return errors.New(ErrInitConfigCode, errors.Alert, []string{"Error while config init", err.Error()}, []string{}, []string{}, []string{})
}
