package config

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrInitConfigCode = "1000"
)

func ErrInitConfig(err error) error {
	return errors.New(ErrInitConfigCode, errors.Alert, []string{"Error while initializing MeshSync configuration. ", err.Error()}, []string{"Missing or outdated CRD. "}, []string{"Missing or outdated CRD."}, []string{"Confirm that meshsyncs custom resource is present in the cluster."})
}
