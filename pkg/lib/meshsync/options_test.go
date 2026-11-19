package meshsync

import (
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/internal/discovery"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPlatformIsKubernetes(t *testing.T) {
	assert.Equal(t, model.PlatformKubernetes, DefautOptions.Platform)
	assert.Contains(t, AllowedPlatforms, DefautOptions.Platform)
}

func TestWithPlatform(t *testing.T) {
	opts := DefautOptions
	WithPlatform("docker")(&opts)

	assert.Equal(t, "docker", opts.Platform)
}

// TestRunRejectsUnknownPlatform checks that Run fails fast, before it touches
// a cluster or a broker, when asked for a platform it does not support.
func TestRunRejectsUnknownPlatform(t *testing.T) {
	log, err := logger.New("meshsync-test", logger.Options{})
	require.NoError(t, err)

	err = Run(log, WithOutputMode(config.OutputModeFile), WithPlatform("docker"))

	require.Error(t, err)
	assert.Equal(t, discovery.ErrUnknownPlatformCode, meshkiterrors.GetCode(err))
}
