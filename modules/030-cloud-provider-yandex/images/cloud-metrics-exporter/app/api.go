// Copyright 2022 Flant JSC
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

package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"exporter/internal/yandex"
)

func openFile(path string) (*os.File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return os.Open(absPath)
}

func InitAPI(api *yandex.CloudApi) error {
	if ApiKeyFilePath != "" {
		apiKeyFile, err := openFile(ApiKeyFilePath)
		if err != nil {
			return err
		}

		defer apiKeyFile.Close()

		apiKeyBytes, err := io.ReadAll(apiKeyFile)
		if err != nil {
			return err
		}

		api.InitWithAPIKey(string(apiKeyBytes))
		return nil
	}

	if ApiKey != "" {
		api.InitWithAPIKey(ApiKey)
		return nil
	}

	return fmt.Errorf("should pass path to service account, pass to file contains APIKey or pass APIKey")
}
