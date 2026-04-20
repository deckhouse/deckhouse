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

package tofu

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type OutputExecutorParams struct {
	RunExecutorParams
}

type OutputExecutor struct {
	params OutputExecutorParams

	logger log.Logger
}

func NewOutputExecutor(params OutputExecutorParams, logger log.Logger) (*OutputExecutor, error) {
	if err := params.validateRunParams(); err != nil {
		return nil, err
	}

	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &OutputExecutor{
		params: params,
		logger: logger,
	}, nil
}

func (e *OutputExecutor) Output(ctx context.Context, opts infrastructure.OutputOpts) ([]byte, error) {
	_, out, err := tofuOutputRun(ctx, e.params.RunExecutorParams, opts)
	return out, err
}

func tofuOutputRun(ctx context.Context, params RunExecutorParams, opts infrastructure.OutputOpts) (*exec.Cmd, []byte, error) {
	args := []string{
		"output",
		"-no-color",
		"-json",
		fmt.Sprintf("-state=%s", opts.StatePath),
	}

	if opts.ShowSensitive {
		args = append(args, "-show-sensitive")
	}

	if len(opts.OutFields) > 0 {
		args = append(args, opts.OutFields...)
	}

	cmd := tofuCmd(ctx, params, "", args...)

	out, err := cmd.Output()

	return cmd, out, err
}
