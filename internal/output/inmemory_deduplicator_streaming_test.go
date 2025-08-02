package output

import (
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestInMemoryDeduplicatorStreamingWriter(t *testing.T) {
	mock := &mockWriter{}
	writer := NewInMemoryDeduplicatorStreamingWriter(mock)

	cfg := config.PipelineConfig{}

	resource1 := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID: "abc-123",
		},
	}

	resource2 := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID: "abc-123", // same UID, should be skipped
		},
	}

	resource3 := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID: "def-456",
		},
	}

	resourceNoUID1 := model.KubernetesResource{
		KubernetesResourceMeta: nil, // no UID
	}

	resourceNoUID2 := model.KubernetesResource{
		KubernetesResourceMeta: nil, // also no UID, treated as unique
	}

	// Write all resources
	assert.NoError(t, writer.Write(resource1, broker.Add, cfg))
	assert.NoError(t, writer.Write(resource2, broker.Add, cfg)) // duplicate UID
	assert.NoError(t, writer.Write(resource3, broker.Add, cfg)) // new UID
	assert.NoError(t, writer.Write(resourceNoUID1, broker.Add, cfg))
	assert.NoError(t, writer.Write(resourceNoUID2, broker.Add, cfg))

	// Expect 4 writes: resource1, resource3, and both no-UID resources
	assert.Len(t, mock.written, 4)

	// Verify ordering and UID correctness
	assert.Equal(t, "abc-123", mock.written[0].KubernetesResourceMeta.UID)
	assert.Equal(t, "def-456", mock.written[1].KubernetesResourceMeta.UID)
	assert.Nil(t, mock.written[2].KubernetesResourceMeta)
	assert.Nil(t, mock.written[3].KubernetesResourceMeta)
}
