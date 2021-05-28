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
	ErrGetObjectCode        = "test_code"
	ErrNewPipelineCode      = "test_code"
	ErrNewInformerCode      = "test_code"
	ErrKubeConfigCode       = "test_code"
	ErrInitRequestCode      = "test_code"
	ErrSubscribeRequestCode = "test_code"
	ErrLogStreamCode        = "test_code"
	ErrCopyBufferCode       = "test_code"
	ErrInvalidRequestCode   = "test_code"

	ErrInvalidRequest = errors.NewDefault(ErrInvalidRequestCode, "Request is invalid")
)

func ErrGetObject(err error) error {
	return errors.NewDefault(ErrGetObjectCode, "Error getting config object", err.Error())
}

func ErrNewPipeline(err error) error {
	return errors.NewDefault(ErrNewPipelineCode, "Error creating pipeline", err.Error())
}

func ErrNewInformer(err error) error {
	return errors.NewDefault(ErrNewInformerCode, "Error initializing informer", err.Error())
}

func ErrKubeConfig(err error) error {
	return errors.NewDefault(ErrKubeConfigCode, "Error creating kubernetes client", err.Error())
}

func ErrInitRequest(err error) error {
	return errors.NewDefault(ErrInitRequestCode, "Error while initializing requests channel", err.Error())
}

func ErrSubscribeRequest(err error) error {
	return errors.NewDefault(ErrSubscribeRequestCode, "Error while subscribing to requests", err.Error())
}

func ErrLogStream(err error) error {
	return errors.NewDefault(ErrLogStreamCode, "Error while open log stream connection", err.Error())
}

func ErrCopyBuffer(err error) error {
	return errors.NewDefault(ErrCopyBufferCode, "Error while copying log buffer", err.Error())
}
