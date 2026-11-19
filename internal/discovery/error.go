package discovery

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrUnknownPlatformCode      = "1016"
	ErrBuildDiscovererCode      = "1017"
	ErrUnsupportedResourceCode  = "1018"
	ErrDiscoverNotSupportedCode = "1019"
)

func ErrUnknownPlatform(platform string) error {
	return errors.New(
		ErrUnknownPlatformCode, errors.Alert,
		[]string{"Unknown discovery platform"},
		[]string{"No discoverer is registered for platform: " + platform},
		[]string{"The requested platform is not supported by this MeshSync build"},
		[]string{"Use one of the supported discovery platforms, for example kubernetes"},
	)
}

func ErrBuildDiscoverer(platform string, err error) error {
	return errors.New(
		ErrBuildDiscovererCode, errors.Alert,
		[]string{"Unable to build the discoverer"},
		[]string{"Building the discoverer for platform " + platform + " failed", err.Error()},
		[]string{"The platform client could not be initialized"},
		[]string{"Check that the platform is reachable and its credentials are valid"},
	)
}

func ErrUnsupportedResource(platform, kind string) error {
	return errors.New(
		ErrUnsupportedResourceCode, errors.Alert,
		[]string{"Resource cannot be written to the output"},
		[]string{"Resource of kind " + kind + " on platform " + platform + " has no Kubernetes payload"},
		[]string{"The output writers only accept Kubernetes resources"},
		[]string{"Use the kubernetes discovery platform"},
	)
}

func ErrDiscoverNotSupported(platform string) error {
	return errors.New(
		ErrDiscoverNotSupportedCode, errors.Alert,
		[]string{"Standalone discovery pass is not supported"},
		[]string{"Platform " + platform + " lists every resource as part of Watch"},
		[]string{"The platform's watch mechanism performs the initial inventory itself"},
		[]string{"Call Watch instead of Discover"},
	)
}
