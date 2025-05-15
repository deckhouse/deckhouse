// Copyright 2024 Flant JSC
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

package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const testRunScript = `#! /bin/bash
echo $@
exit 0`

func TestScriptExecute(t *testing.T) {
	t.SkipNow()

	s := require.New(t)
	scriptPath := filepath.Join(os.TempDir(), "test_run.sh")
	err := os.WriteFile(scriptPath, []byte(testRunScript), 0774)
	s.NoError(err)
	t.Cleanup(func() {
		_ = os.Remove(scriptPath)
	})

	script := NewScript(scriptPath, "arg 1", "arg 2")
	stdout, err := script.Execute(context.Background())
	s.NoError(err)
	s.Equal(string(stdout), "arg 1 arg 2")
}
