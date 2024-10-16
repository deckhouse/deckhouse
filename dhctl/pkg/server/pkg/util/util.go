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

package util

import (
	"fmt"
	"os"
	"strconv"
)

func WriteDefaultTempFile(data []byte) (string, func() error, error) {
	return WriteTempFile("", "*", func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}

func WriteTempFile(dir, pathPattern string, writer func(*os.File) error) (string, func() error, error) {
	f, err := os.CreateTemp(dir, pathPattern)
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}
	cleanup := func() error {
		return os.RemoveAll(f.Name())
	}

	if err := writer(f); err != nil {
		defer cleanup()
		return "", nil, fmt.Errorf("writing temp file: %w", err)
	}

	return f.Name(), cleanup, nil
}

func ErrToString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func PortToString(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

func StringToBytes(data string) []byte {
	if data == "" {
		return nil
	}
	return []byte(data)
}
