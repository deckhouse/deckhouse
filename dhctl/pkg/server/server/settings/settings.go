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

package settings

import (
	"fmt"
	"path"
	"time"
)

type ServerGeneralParams struct {
	Network string
	Address string
	TmpDir  string
}

func (p *ServerGeneralParams) Validate() error {
	if p.Network == "" {
		return fmt.Errorf("network is required")
	}
	if p.Address == "" {
		return fmt.Errorf("address is required")
	}

	if err := ValidateTmpPath(p.TmpDir); err != nil {
		return err
	}

	return nil
}

type ServerSingleshotParams struct {
	ServerGeneralParams
}

func (p *ServerSingleshotParams) Validate() error {
	return p.ServerGeneralParams.Validate()
}

type ServerParams struct {
	ServerGeneralParams

	ParallelTasksLimit         int
	RequestsCounterMaxDuration time.Duration
}

func (p *ServerParams) Validate() error {
	return p.ServerGeneralParams.Validate()
}

func ValidateTmpPath(fullPath string) error {
	tmpDir := path.Clean(fullPath)

	if tmpDir == "" {
		return fmt.Errorf("tmpdir is required")
	}

	if tmpDir == "/" {
		return fmt.Errorf("tmpdir should not be /")
	}

	return nil
}
