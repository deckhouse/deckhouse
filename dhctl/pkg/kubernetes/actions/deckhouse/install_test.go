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

package deckhouse

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestDeckhouseInstall(t *testing.T) {
	log.InitLogger("simple")
	fakeClient := client.NewFakeKubernetesClient()

	err := os.WriteFile("/deckhouse/version", []byte("1.54.1"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	tests := []struct {
		name    string
		test    func() error
		wantErr bool
	}{
		{
			"Empty config",
			func() error {
				return CreateDeckhouseManifests(fakeClient, &Config{})
			},
			false,
		},
		{
			"Double install",
			func() error {
				err := CreateDeckhouseManifests(fakeClient, &Config{})
				if err != nil {
					return err
				}
				return CreateDeckhouseManifests(fakeClient, &Config{})
			},
			false,
		},
		{
			"With docker cfg",
			func() error {
				err := CreateDeckhouseManifests(fakeClient, &Config{
					Registry: config.RegistryData{DockerCfg: "YW55dGhpbmc="},
				})
				if err != nil {
					return err
				}
				s, err := fakeClient.CoreV1().Secrets("d8-system").Get(context.TODO(), "deckhouse-registry", metav1.GetOptions{})
				if err != nil {
					return err
				}

				dockercfg := s.Data[".dockerconfigjson"]
				if string(dockercfg) != "anything" {
					return fmt.Errorf(".dockercfg data: %s", dockercfg)
				}
				return nil
			},
			false,
		},
		{
			"With secrets",
			func() error {
				conf := Config{
					ClusterConfig:         []byte(`test`),
					ProviderClusterConfig: []byte(`test`),
					TerraformState:        []byte(`test`),
				}
				err := CreateDeckhouseManifests(fakeClient, &conf)
				if err != nil {
					return err
				}
				return nil
			},
			false,
		},
	}

	for _, tc := range tests {
		err := tc.test()

		if err != nil && !tc.wantErr {
			t.Errorf("%s: %v", tc.name, err)
		}

		if err == nil && tc.wantErr {
			t.Errorf("%s: expected error, didn't get one", tc.name)
		}
	}
}

func TestDeckhouseInstallWithDevBranch(t *testing.T) {
	fakeClient := client.NewFakeKubernetesClient()

	err := os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	err = CreateDeckhouseManifests(fakeClient, &Config{
		DevBranch: "pr1111",
	})

	require.NoError(t, err)
}
