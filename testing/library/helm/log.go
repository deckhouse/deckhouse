// Copyright 2021 Flant JSC
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

package helm

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Catch warning logs messages and filter them
// https://github.com/helm/helm/issues/7019
type FilteredHelmWriter struct {
	Writer io.Writer
}

var _ io.Writer = (*FilteredHelmWriter)(nil)

func (w *FilteredHelmWriter) Write(p []byte) (int, error) {
	builder := strings.Builder{}

	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.Contains(line, "found symbolic link in path") {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	result := strings.TrimSuffix(builder.String(), "\n")
	n, err := w.Writer.Write([]byte(result))
	if err != nil {
		return n, fmt.Errorf("write: %w", err)
	}
	return n, nil
}
