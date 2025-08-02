package output

import (
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshsync/internal/config"
	"github.com/meshery/meshsync/pkg/model"
	"github.com/stretchr/testify/assert"
)

// mockWriter is a mock implementation of Writer that records written objects
type mockWriter struct {
	written []model.KubernetesResource
	events  []broker.EventType
	configs []config.PipelineConfig
}

func (m *mockWriter) Write(obj model.KubernetesResource, evtype broker.EventType, cfg config.PipelineConfig) error {
	m.written = append(m.written, obj)
	m.events = append(m.events, evtype)
	m.configs = append(m.configs, cfg)
	return nil
}

func TestInMemoryDeduplicatorWriter(t *testing.T) {
	mock := &mockWriter{}
	writer := NewInMemoryDeduplicatorWriter(mock)

	cfg := config.PipelineConfig{}

	// UID-based resources
	resource1 := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID: "uid-1",
		},
	}
	resource1Update := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID: "uid-1",
		},
	}

	resource2 := model.KubernetesResource{
		KubernetesResourceMeta: &model.KubernetesResourceObjectMeta{
			UID: "uid-2",
		},
	}

	// Resources without UID
	resourceNoUID1 := model.KubernetesResource{
		KubernetesResourceMeta: nil,
	}
	resourceNoUID2 := model.KubernetesResource{
		KubernetesResourceMeta: nil,
	}

	// Simulate writes
	assert.NoError(t, writer.Write(resource1, broker.Add, cfg))
	assert.NoError(t, writer.Write(resource1Update, broker.Update, cfg)) // should overwrite resource1
	assert.NoError(t, writer.Write(resource2, broker.Add, cfg))
	assert.NoError(t, writer.Write(resourceNoUID1, broker.Add, cfg))
	assert.NoError(t, writer.Write(resourceNoUID2, broker.Add, cfg))

	// Flush and verify results
	err := writer.Flush()
	assert.NoError(t, err)

	// Expect: 2 unique UID-based writes (resource1Update & resource2)
	//         2 separate no-UID writes
	assert.Len(t, mock.written, 4)

	// Check UID of the written resources
	uidMap := map[string]bool{}
	noUIDCount := 0

	for _, r := range mock.written {
		if r.KubernetesResourceMeta == nil {
			noUIDCount++
		} else {
			uidMap[r.KubernetesResourceMeta.UID] = true
		}
	}

	assert.Equal(t, 2, len(uidMap))
	assert.Contains(t, uidMap, "uid-1")
	assert.Contains(t, uidMap, "uid-2")
	assert.Equal(t, 2, noUIDCount)
}
