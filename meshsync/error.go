// Copyright 2020 Layer5, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package meshsync

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrSetupClusterCode     = "test_code"
	ErrSetupIstioCode       = "test_code"
	ErrKubeConfigCode       = "test_code"
	ErrNewDiscoveryCode     = "test_code"
	ErrNewInformerCode      = "test_code"
	ErrNewKubeClientCode    = "test_code"
	ErrNewDynClientCode     = "test_code"
	ErrNewMesheryClientCode = "test_code"
)

func ErrSetupCluster(err error) error {
	return errors.NewDefault(ErrSetupClusterCode, "Error seting up cluster", err.Error())
}
func ErrSetupIstio(err error) error {
	return errors.NewDefault(ErrSetupIstioCode, "Error setting up istio", err.Error())
}
func ErrKubeConfig(err error) error {
	return errors.NewDefault(ErrKubeConfigCode, "Error initializing kubeconfig", err.Error())
}
func ErrNewDiscovery(err error) error {
	return errors.NewDefault(ErrNewDiscoveryCode, "Error initializing discovery client", err.Error())
}
func ErrNewInformer(err error) error {
	return errors.NewDefault(ErrNewInformerCode, "Error initializing informer client", err.Error())
}
func ErrNewKubeClient(err error) error {
	return errors.NewDefault(ErrNewKubeClientCode, "Error initializing kube client", err.Error())
}
func ErrNewDynClient(err error) error {
	return errors.NewDefault(ErrNewDynClientCode, "Error initializing dynamic kube client", err.Error())
}
func ErrNewMesheryClient(err error) error {
	return errors.NewDefault(ErrNewMesheryClientCode, "Error initializing meshery kube client", err.Error())
}
