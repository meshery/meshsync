package config

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrInitConfigCode = "test_code"
)

func ErrInitConfig(err error) error {
	return errors.NewDefault(ErrInitConfigCode, fmt.Sprintf("Error while config init", err.Error()))
}
