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

package grpc

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrRequestInvalid = errors.NewDefault("603", "Apply Request invalid")
)

func ErrPanic(r interface{}) error {
	return errors.NewDefault(errors.ErrPanic, fmt.Sprintf("%v", r))
}

func ErrGrpcListener(err error) error {
	return errors.NewDefault(errors.ErrGrpcListener, fmt.Sprintf("Error during grpc listener initialization : %v", err))
}

func ErrGrpcServer(err error) error {
	return errors.NewDefault(errors.ErrGrpcServer, fmt.Sprintf("Error during grpc server initialization : %v", err))
}
