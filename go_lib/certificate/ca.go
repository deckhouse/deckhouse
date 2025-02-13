/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certificate

import (
	"bytes"
	"log"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

type Authority struct {
	Key  string `json:"key"`
	Cert string `json:"cert"`
}

func GenerateCA(logger go_hook.Logger, cn string, options ...Option) (Authority, error) {
	request := &csr.CertificateRequest{
		CN: cn,
		CA: &csr.CAConfig{
			Expiry: "87600h",
		},
		KeyRequest: &csr.KeyRequest{
			A: "ecdsa",
			S: 256,
		},
	}

	for _, option := range options {
		option(request)
	}

	// Catch cfssl output and show it only if error is occurred.
	var buf bytes.Buffer
	logWriter := log.Writer()

	log.SetOutput(&buf)
	defer log.SetOutput(logWriter)

	ca, _, key, err := initca.New(request)
	if err != nil {
		logger.Error(buf.String())
	}

	return Authority{Cert: string(ca), Key: string(key)}, err
}
