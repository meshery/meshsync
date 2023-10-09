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
		input           []model.Object
		maxItmsPerSlice int
		expectedOutput  [][]model.Object
	}{
		{
			name:            "test with 0 items",
			input:           []model.Object{},
			maxItmsPerSlice: 10,
			expectedOutput:  [][]model.Object{},
		},

		{
			name: "test with 1 item",
			input: []model.Object{
				{
					Kind: "test",
				},
			},
			maxItmsPerSlice: 10,
			expectedOutput: [][]model.Object{
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
