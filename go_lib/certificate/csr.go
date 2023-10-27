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
	"github.com/sirupsen/logrus"
)

func GenerateCSR(logger *logrus.Entry, cn string, options ...Option) (csrPEM, key []byte, err error) {
	request := &csr.CertificateRequest{
		CN:         cn,
		KeyRequest: csr.NewKeyRequest(),
	}

	for _, option := range options {
		option(request)
	}

	g := &csr.Generator{Validator: Validator}

	// Catch cfssl output and show it only if error is occurred.
	var buf bytes.Buffer
	logWriter := log.Writer()

	log.SetOutput(&buf)
	defer log.SetOutput(logWriter)

	csrPEM, key, err = g.ProcessRequest(request)
	if err != nil && logger != nil {
		logger.Errorln(buf.String())
	}

	return
}

// Validator does nothing and will never return an error. It exists because creating a
// csr.Generator requires a Validator.
func Validator(_ *csr.CertificateRequest) error {
	return nil
}
