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

var (
	ErrGetObjectCode        = "1004"
	ErrNewPipelineCode      = "1005"
	ErrNewInformerCode      = "1006"
	ErrKubeConfigCode       = "1007"
	ErrInitRequestCode      = "1008"
	ErrSubscribeRequestCode = "1009"
	ErrLogStreamCode        = "1010"
	ErrCopyBufferCode       = "1011"
	ErrInvalidRequestCode   = "1012"
	ErrExecTerminalCode     = "1013"

	ErrInvalidRequest = errors.New(ErrInvalidRequestCode, errors.Alert, []string{"Request is invalid"}, []string{}, []string{"Stale request on the broker"}, []string{"Make sure the request format is correctly configured"})
)

func ErrGetObject(err error) error {
	return errors.New(ErrGetObjectCode, errors.Alert, []string{"Error getting config object"}, []string{err.Error()}, []string{"Config doesnt exist"}, []string{"Check application config is configured correct or restart the server"})
}

func ErrNewPipeline(err error) error {
	return errors.New(ErrNewPipelineCode, errors.Alert, []string{"Error creating pipeline"}, []string{err.Error()}, []string{"Pipeline step failed"}, []string{"Investigate on the respective pipeline step that has failed to figure the cause"})
}

func ErrNewInformer(err error) error {
	return errors.New(ErrNewInformerCode, errors.Alert, []string{"Error initializing informer"}, []string{err.Error()}, []string{"Resource is invalid or doesnt exist"}, []string{"Make sure to input the existing valid resource"})
}

func ErrKubeConfig(err error) error {
	return errors.New(ErrKubeConfigCode, errors.Alert, []string{"Error creating kubernetes client"}, []string{err.Error()}, []string{"Kubernetes config is invalid or APi server is not reachable"}, []string{"Make sure to upload a valid kubernetes config", "Make sure kubernetes API server is reachable"})
}

func ErrInitRequest(err error) error {
	return errors.New(ErrInitRequestCode, errors.Alert, []string{"Error while initializing requests channel"}, []string{err.Error()}, []string{"Application resource deficit"}, []string{"Make sure meshsync has enough resources to create channels"})
}

func ErrSubscribeRequest(err error) error {
	return errors.New(ErrSubscribeRequestCode, errors.Alert, []string{"Error while subscribing to requests"}, []string{err.Error()}, []string{"Broker resource deficit"}, []string{"Make sure Broker has enough resources to create channels"})
}

func ErrLogStream(err error) error {
	return errors.New(ErrLogStreamCode, errors.Alert, []string{"Error while open log stream connection"}, []string{err.Error()}, []string{"requested Resource could be invalid"}, []string{"Make sure the requested resource is valid and existing"})
}

func ErrExecTerminal(err error) error {
	return errors.New(ErrExecTerminalCode, errors.Alert, []string{"Error while opening a terminal session"}, []string{err.Error()}, []string{"requested Resource could be invalid"}, []string{"Make sure the requested resource is valid and existing"})
}

func ErrCopyBuffer(err error) error {
	return errors.New(ErrCopyBufferCode, errors.Alert, []string{"Error while copying log buffer"}, []string{err.Error()}, []string{}, []string{})
}
