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

	ErrInvalidRequest = errors.New(ErrInvalidRequestCode, errors.Alert, []string{"Request is invalid"}, []string{}, []string{}, []string{})
)

func ErrGetObject(err error) error {
	return errors.New(ErrGetObjectCode, errors.Alert, []string{"Error getting config object", err.Error()}, []string{}, []string{}, []string{})
}

func ErrNewPipeline(err error) error {
	return errors.New(ErrNewPipelineCode, errors.Alert, []string{"Error creating pipeline", err.Error()}, []string{}, []string{}, []string{})
}

func ErrNewInformer(err error) error {
	return errors.New(ErrNewInformerCode, errors.Alert, []string{"Error initializing informer", err.Error()}, []string{}, []string{}, []string{})
}

func ErrKubeConfig(err error) error {
	return errors.New(ErrKubeConfigCode, errors.Alert, []string{"Error creating kubernetes client", err.Error()}, []string{}, []string{}, []string{})
}

func ErrInitRequest(err error) error {
	return errors.New(ErrInitRequestCode, errors.Alert, []string{"Error while initializing requests channel", err.Error()}, []string{}, []string{}, []string{})
}

func ErrSubscribeRequest(err error) error {
	return errors.New(ErrSubscribeRequestCode, errors.Alert, []string{"Error while subscribing to requests", err.Error()}, []string{}, []string{}, []string{})
}

func ErrLogStream(err error) error {
	return errors.New(ErrLogStreamCode, errors.Alert, []string{"Error while open log stream connection", err.Error()}, []string{}, []string{}, []string{})
}

func ErrCopyBuffer(err error) error {
	return errors.New(ErrCopyBufferCode, errors.Alert, []string{"Error while copying log buffer", err.Error()}, []string{}, []string{}, []string{})
}
