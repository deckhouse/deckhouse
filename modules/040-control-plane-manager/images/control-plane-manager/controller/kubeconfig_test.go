/*
Copyright 2023 Flant JSC

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

package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func TestLoadKubeconfig(t *testing.T) {
	k, err := loadKubeconfig("testdata/kubeconfig.conf")
	if err != nil {
		t.Fatal(err)
	}

	certData, err := base64.StdEncoding.DecodeString(k.Users[0].User.ClientCertificateData)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certData)
	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
}
