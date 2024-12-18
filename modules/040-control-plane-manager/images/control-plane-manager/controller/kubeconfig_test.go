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
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func TestLoadAndParseKubeconfig(t *testing.T) {
	k, err := loadKubeconfig("testdata/kubeconfig.conf")
	if err != nil {
		t.Fatal(err)
	}

	if len(k.AuthInfos[0].AuthInfo.ClientCertificateData) == 0 {
		t.Fatal("client certificate data is empty")
	}

	block, _ := pem.Decode(k.AuthInfos[0].AuthInfo.ClientCertificateData)
	if len(block.Bytes) == 0 {
		t.Fatal("cannot pem decode block")
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

}

func TestValidateKubeconfig(t *testing.T) {
	err := validateKubeconfig("testdata/kubeconfig.conf", "testdata/kubeconfig_tmp.conf")
	if err != nil {
		if strings.Contains(err.Error(), "is expiring in less than 30 days") {
			if !errors.Is(err, ErrCertExpiringSoon) {
				t.Fatalf("expected remove to be true when certificate is expiring, got %v", err)
			}
			t.Log("Warning: client certificate is expiring soon, kubeconfig will be recreated.")
		} else {
			t.Fatal(err)
		}
	}
}

func TestCheckEtcdManifest(t *testing.T) {
	os.Setenv("D8_TESTS", "yes")
	config = &Config{
		NodeName: "dev-master-0",
		MyIP:     "192.168.199.39",
	}
	err := checkEtcdManifest()
	if err != nil {
		t.Fatal(err)
	}
}
