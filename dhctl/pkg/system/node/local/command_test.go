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

func TestCommandOutput(t *testing.T) {
	s := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), "test")
	tmpFile, err := os.Create(testFilePath)
	s.NoError(err)
	t.Cleanup(func() {
		_ = os.Remove(testFilePath)
	})

	_, err = tmpFile.WriteString("Hello world")
	s.NoError(err)

	cmd := NewCommand("cat", testFilePath)
	stdout, _, err := cmd.Output(context.Background())
	s.NoError(err)
	s.Equal("Hello world", string(stdout))
}

func TestCommandCombinedOutput(t *testing.T) {
	s := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), "test")
	tmpFile, err := os.Create(testFilePath)
	s.NoError(err)
	t.Cleanup(func() {
		_ = os.Remove(testFilePath)
	})

	_, err = tmpFile.WriteString("Hello world")
	s.NoError(err)

	cmd := NewCommand("cat", testFilePath)
	stdout, err := cmd.CombinedOutput(context.Background())
	s.NoError(err)
	s.Equal("Hello world", string(stdout))
}

func TestCommandRun(t *testing.T) {
	s := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), "test")
	tmpFile, err := os.Create(testFilePath)
	s.NoError(err)
	t.Cleanup(func() {
		_ = os.Remove(testFilePath)
	})

	_, err = tmpFile.WriteString("Hello world")
	s.NoError(err)

	cmd := NewCommand("cat", testFilePath)
	err = cmd.Run(context.Background())
	s.NoError(err)
	s.Equal("Hello world", string(cmd.StdoutBytes()))
	s.Nil(cmd.StderrBytes())
}

func TestCommandPipe(t *testing.T) {
	s := require.New(t)

	cmd := NewCommand("bash", "-c", `echo "Goodbye world" | sed "s/Goodbye/Hello/g"`)
	s.NoError(cmd.Run(context.Background()))
	s.Equal("Hello world", string(cmd.StdoutBytes()))
	s.Nil(cmd.StderrBytes())
}
