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

package jwt

import (
	"fmt"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/jwt"
)

func GenJWT(privateKeyPath string, claims map[string]string, ttl time.Duration) error {
	privKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("os.ReadFile: %w", err)
	}

	tokenString, err := jwt.GenerateJWT(privKeyBytes, claims, ttl)
	if err != nil {
		return fmt.Errorf("jwt.GenerateJWT: %w", err)
	}

	fmt.Fprint(os.Stdout, tokenString)
	return nil
}
