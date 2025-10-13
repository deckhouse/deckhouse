// Copyright 2025 Flant JSC
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

package terraform

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type OutputExecutorParams struct {
	RunExecutorParams
}

type OutputExecutor struct {
	params OutputExecutorParams

	logger log.Logger
}

func NewOutputExecutor(params OutputExecutorParams, logger log.Logger) *OutputExecutor {
	return &OutputExecutor{
		params: params,
		logger: logger,
	}
}

func (e *OutputExecutor) Output(ctx context.Context, statePath string, outFielda ...string) (result []byte, err error) {
	_, out, err := terraformOutputRun(ctx, e.params.RunExecutorParams, statePath, outFielda...)

	return out, err
}

func terraformOutputRun(ctx context.Context, params RunExecutorParams, statePath string, outFielda ...string) (*exec.Cmd, []byte, error) {
	args := []string{
		"output",
		"-no-color",
		"-json",
		fmt.Sprintf("-state=%s", statePath),
	}
	if len(outFielda) > 0 {
		args = append(args, outFielda...)
	}

	cmd := terraformCmd(ctx, params, args...)

	out, err := cmd.Output()

	return cmd, out, err
}
