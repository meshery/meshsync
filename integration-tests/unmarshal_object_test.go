package tests

import (
	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/utils"
	"github.com/layer5io/meshsync/pkg/model"
)

func unmarshalObject(object interface{}) (model.KubernetesResource, error) {
	objectJSON, _ := utils.Marshal(object)
	obj := model.KubernetesResource{}
	err := encoding.Unmarshal([]byte(objectJSON), &obj)
	return obj, err
}
