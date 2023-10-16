package meshsync

import (
	"reflect"
	"testing"

	"github.com/layer5io/meshsync/pkg/model"
)

// TestSplitIntoMultipleSlices tests the splitIntoMultipleSlices function
// by providing different input test cases and comparing the output with the expected output.
func TestSplitIntoMultipleSlices(t *testing.T) {
	testCases := []struct {
		name            string
		input           []model.KubernetesObject
		maxItmsPerSlice int
		expectedOutput  [][]model.KubernetesObject
	}{
		{
			name:            "test with 0 items",
			input:           []model.KubernetesObject{},
			maxItmsPerSlice: 10,
			expectedOutput:  [][]model.KubernetesObject{},
		},

		{
			name: "test with 1 item",
			input: []model.KubernetesObject{
				{
					Kind: "test",
				},
			},
			maxItmsPerSlice: 10,
			expectedOutput: [][]model.KubernetesObject{
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
