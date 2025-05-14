package tests

import (
	"github.com/layer5io/meshkit/encoding"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshsync/pkg/model"
)

func unmarshalObject(object interface{}) (model.KubernetesResource, error) {
	objectJSON, _ := utils.Marshal(object)
	obj := model.KubernetesResource{}
	err := encoding.Unmarshal([]byte(objectJSON), &obj)
	return obj, err
}
