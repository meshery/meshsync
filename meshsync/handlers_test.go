package meshsync

import (
	"reflect"
	"testing"

	"github.com/meshery/meshsync/pkg/model"
)

// TestSplitIntoMultipleSlices tests the splitIntoMultipleSlices function
// by providing different input test cases and comparing the output with the expected output.
func TestSplitIntoMultipleSlices(t *testing.T) {
	testCases := []struct {
		name            string
		input           []model.KubernetesResource
		maxItmsPerSlice int
		expectedOutput  [][]model.KubernetesResource
	}{
		{
			name:            "test with 0 items",
			input:           []model.KubernetesResource{},
			maxItmsPerSlice: 10,
			expectedOutput:  [][]model.KubernetesResource{},
		},

		{
			name: "test with 1 item",
			input: []model.KubernetesResource{
				{
					Kind: "test",
				},
			},
			maxItmsPerSlice: 10,
			expectedOutput: [][]model.KubernetesResource{
				{
					{
						Kind: "test",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := splitIntoMultipleSlices(tc.input, tc.maxItmsPerSlice)
			if !reflect.DeepEqual(output, tc.expectedOutput) {
				t.Errorf("expected %v, but got %v", tc.expectedOutput, output)
			}
		})
	}
}
