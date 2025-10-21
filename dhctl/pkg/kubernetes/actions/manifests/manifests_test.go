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

package manifests

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func compareDeployments(t *testing.T, depl1, depl2 *appsv1.Deployment) {
	depl1Yaml, err := yaml.Marshal(depl1)
	if err != nil {
		t.Errorf("deployment from struct unmarshal: %v", err)
	}
	depl2Yaml, err := yaml.Marshal(depl2)
	if err != nil {
		t.Errorf("deployment from yaml unmarshal: %v", err)
	}

	depl1Lines := strings.Split(string(depl1Yaml), "\n")
	depl2Lines := strings.Split(string(depl2Yaml), "\n")

	if len(depl1Lines) != len(depl2Lines) {
		t.Logf("depl1 lines: %d, depl2 lines: %d", len(depl1Lines), len(depl2Lines))
	}

	i := 0
	diff := 0

	builder := strings.Builder{}
	builder.WriteString("\n")
	for {
		l1 := ""
		l2 := ""

		if i < len(depl1Lines) {
			l1 = depl1Lines[i]
		}

		if i < len(depl2Lines) {
			l2 = depl2Lines[i]
		}

		mark := " "
		if l1 != l2 {
			mark = "!"
			diff++
		}

		builder.WriteString(fmt.Sprintf("%s %-70s %-35s\n", mark, l1, l2))

		i++
		if i >= len(depl1Lines) && i >= len(depl2Lines) {
			break
		}
	}

	if diff > 0 {
		t.Fatalf("%s", builder.String())
	}
}

func Test_struct_vs_unmarshal(t *testing.T) {
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	err = os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	params := DeckhouseDeploymentParams{
		Registry:         "registry.example.com/deckhouse:master",
		LogLevel:         "debug",
		Bundle:           "default",
		DeployTime:       time.Now(),
	}

	depl1 := DeckhouseDeployment(params)
	depl2 := DeckhouseDeployment(params)

	compareDeployments(t, depl1, depl2)
}

func Test_DeployTime(t *testing.T) {
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	err = os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	paramsGet := func() DeckhouseDeploymentParams {
		return DeckhouseDeploymentParams{
			Registry:         "registry.example.com/deckhouse:master",
			LogLevel:         "debug",
			Bundle:           "default",
		}
	}

	t.Run("set non zero deploy time if DeployTime param does not pass", func(t *testing.T) {
		p := paramsGet()

		depl := DeckhouseDeployment(p)
		tm := GetDeckhouseDeployTime(depl)

		require.False(t, tm.IsZero())
	})

	t.Run("set same deploy time as DeployTime from param if it present", func(t *testing.T) {
		expectTime, _ := time.Parse(time.RFC822, "02 Jan 06 15:04 MST")

		p := paramsGet()
		p.DeployTime = expectTime

		depl := DeckhouseDeployment(p)
		tm := GetDeckhouseDeployTime(depl)

		require.False(t, tm.IsZero())
		require.Equal(t, tm.UnixNano(), expectTime.UnixNano())
	})
}

func Test_DoNotMutateDeployment(t *testing.T) {
	err := os.Setenv("DHCTL_TEST", "yes")
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("DHCTL_TEST")
	}()

	err = os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
	if err != nil {
		panic(err)
	}

	defer func() {
		os.Remove("/deckhouse/version")
	}()

	tc := []struct {
		name   string
		params DeckhouseDeploymentParams
	}{
		{
			name: "Kube Service",
			params: DeckhouseDeploymentParams{
				Registry:         "registry.example.com/deckhouse:master",
				LogLevel:         "debug",
				Bundle:           "default",
				KubeadmBootstrap: true,
			},
		},
		{
			name: "Master NodeSelector",
			params: DeckhouseDeploymentParams{
				Registry:           "registry.example.com/deckhouse:master",
				LogLevel:           "debug",
				Bundle:             "default",
				MasterNodeSelector: true,
			},
		},
		{
			name: "All in",
			params: DeckhouseDeploymentParams{
				Registry:           "registry.example.com/deckhouse:master",
				LogLevel:           "debug",
				Bundle:             "default",
				KubeadmBootstrap:   true,
				MasterNodeSelector: true,
			},
		},
		{
			name: "With time",
			params: DeckhouseDeploymentParams{
				Registry:           "registry.example.com/deckhouse:master",
				LogLevel:           "debug",
				Bundle:             "default",
				DeployTime:         time.Now(),
				KubeadmBootstrap:   true,
				MasterNodeSelector: true,
			},
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			depl := DeckhouseDeployment(c.params)

			if c.params.DeployTime.IsZero() {
				c.params.DeployTime = time.Now()
			}

			newDepl := ParameterizeDeckhouseDeployment(depl, c.params)
			compareDeployments(t, depl, newDepl)
		})
	}
}
