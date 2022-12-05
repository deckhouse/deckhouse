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

package hooks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryversion "k8s.io/apimachinery/pkg/version"

	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/module"
)

const (
	kubeEndpointsSnap         = "endpoinds"
	kubeAPIServK8sLabeledSnap = "apiserver-k8s-app"
	kubeAPIServCPLabeledSnap  = "apiserver-cp"
)

const kubeVersionFileName = "/tmp/kubectl_version"

const apiServerNs = "kube-system"

// versionHTTPClient is used to validate that tls certificate DNS name contains kubernetes service cluster ip
var (
	versionHTTPClient d8http.Client
	once              sync.Once
)

func apiServerK8sAppLabels() map[string]string {
	return map[string]string{
		"k8s-app": "kube-apiserver",
	}
}

func apiServerControlPlaneLabels() map[string]string {
	return map[string]string{
		"component": "kube-apiserver",
		"tier":      "control-plane",
	}
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		// it needs for change apiserver
		{
			Name:       kubeAPIServCPLabeledSnap,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1meta.LabelSelector{
				MatchLabels: apiServerControlPlaneLabels(),
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{apiServerNs},
				},
			},
			FilterFunc: applyAPIServerPodFilter,
		},
		// it needs for change apiserver
		{
			Name:       kubeAPIServK8sLabeledSnap,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1meta.LabelSelector{
				MatchLabels: apiServerK8sAppLabels(),
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{apiServerNs},
				},
			},
			FilterFunc: applyAPIServerPodFilter,
		},

		{
			Name:       kubeEndpointsSnap,
			ApiVersion: "v1",
			Kind:       "Endpoints",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"default"},
				},
			},
			FilterFunc: applyEndpointsAPIServerFilter,
		},
	},
}, k8sVersions)

func applyAPIServerPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// creationTimestamp needs for run hook on restart pod (name of apiserver not contains generated part)
	// if use only name then checksum will be identical for all time
	return fmt.Sprintf("%s/%d", obj.GetName(), obj.GetCreationTimestamp().UnixNano()), nil
}

func applyEndpointsAPIServerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var endpoints v1core.Endpoints

	err := sdk.FromUnstructured(obj, &endpoints)
	if err != nil {
		return nil, err
	}

	addresses := make([]string, 0)

	for _, s := range endpoints.Subsets {
		ports := make([]int32, 0)
		for _, port := range s.Ports {
			if port.Name == "https" {
				ports = append(ports, port.Port)
			}
		}

		for _, addrObj := range s.Addresses {
			for _, port := range ports {
				addr := fmt.Sprintf("%s:%d", addrObj.IP, port)
				addresses = append(addresses, addr)
			}
		}
	}
	return addresses, nil
}

// getKubeVersionForServer
// we do not use Discovery().ServerVersion() because it returns one version from one api server
// (probably it is master-node with deckhouse pod)
// yes, in one time k8s may have different versions on masters at one time
// That doesn't suit us.
// Therefore, we need to request all api servers, get versions and choice minimal
func getKubeVersionForServer(endpoint string, cl d8http.Client) (*semver.Version, error) {
	url := fmt.Sprintf("https://%s/version?timeout=5s", endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	err = d8http.SetKubeAuthToken(req)
	if err != nil {
		return nil, err
	}

	res, err := cl.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("k8s version: incorrect response code: %v", res.Status)
	}

	var info apimachineryversion.Info
	err = json.NewDecoder(res.Body).Decode(&info)
	if err != nil {
		return nil, err
	}

	ver, err := semver.NewVersion(info.GitVersion)
	if err != nil {
		return nil, err
	}

	return ver, nil
}

func apiServerEndpoints(input *go_hook.HookInput) ([]string, error) {
	endpointsSnap := input.Snapshots[kubeEndpointsSnap]
	serverK8sLabeledSnap := input.Snapshots[kubeAPIServK8sLabeledSnap]
	serverCPLabeledSnap := input.Snapshots[kubeAPIServCPLabeledSnap]

	podsCnt := 0
	if c := len(serverK8sLabeledSnap); c > 0 {
		podsCnt = c
	} else if c := len(serverCPLabeledSnap); c > 0 {
		podsCnt = c
	} else {
		input.LogEntry.Infoln("k8s version. Pods snapshots is empty")
	}

	var endpoints []string
	if len(endpointsSnap) > 0 {
		endpointsRaw := endpointsSnap[0]
		endpoints = endpointsRaw.([]string)
	} else {
		input.LogEntry.Infoln("k8s version. Endpoints snapshots is empty")
	}
	endpointsCnt := len(endpoints)

	if endpointsCnt == 0 && podsCnt == 0 {
		input.LogEntry.Infoln("k8s version. Endpoints and pods not found. Skip")
		return nil, nil
	}

	controlPlaneEnabled := module.IsEnabled("control-plane-manager", input)

	if controlPlaneEnabled && podsCnt != endpointsCnt {
		msg := fmt.Sprintf("Not found k8s versions. Pods(%v) != Endpoints (%v) count", podsCnt, endpointsCnt)

		versions := input.Values.Get("global.discovery.kubernetesVersions")
		minVer := input.Values.Get("global.discovery.kubernetesVersion")
		// need return err for retry if k8s versions not found
		// in otherwise we need skip it
		// for example, api server pods can restart and we will get errors here
		// in bash hook we don't subscribe for deleting pods
		// it is emulating this behaviour
		if !versions.Exists() || !minVer.Exists() {
			return nil, fmt.Errorf(msg)
		}

		input.LogEntry.Warnln(msg)

		return nil, nil
	}

	return endpoints, nil
}

func k8sVersions(input *go_hook.HookInput) error {
	input.LogEntry.Infoln("k8s version. Start discovery")
	endpoints, err := apiServerEndpoints(input)
	if err != nil {
		return err
	}
	if endpoints == nil {
		return nil
	}

	// Dedicated client for version discovery is required because cloud providers tend to issue certificates only for
	// cluster IP, yet Deckhouse requests each endpoint separately. Certificate check will fail in this case.
	//
	// ServerName option allows Deckhouse to check, that certificate is issued for the kubernetes service dns name
	// even if it requests apiserver endpoint.
	once.Do(func() {
		if versionHTTPClient != nil {
			return
		}
		contentCA, _ := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")

		versionHTTPClient = d8http.NewClient(
			d8http.WithTLSServerName("kubernetes.default.svc"),
			d8http.WithAdditionalCACerts([][]byte{contentCA}),
		)
	})

	versions := make([]string, 0)
	var minVer *semver.Version

	for _, endpoint := range endpoints {
		ver, err := getKubeVersionForServer(endpoint, versionHTTPClient)
		if err != nil {
			return err
		}

		if minVer == nil || ver.LessThan(minVer) {
			minVer = ver
		}
		versions = append(versions, ver.String())
	}

	if len(versions) == 0 {
		input.LogEntry.Infoln("k8s version. Versions is empty. Skip")
		return nil
	}

	minVerStr := minVer.String()

	err = ioutil.WriteFile(kubeVersionFileName, []byte(minVerStr), os.FileMode(0644))
	if err != nil {
		return err
	}
	input.Values.Set("global.discovery.kubernetesVersions", versions)
	input.Values.Set("global.discovery.kubernetesVersion", minVerStr)

	requirements.SaveValue("global.discovery.kubernetesVersion", minVerStr)
	input.LogEntry.Infof("k8s version was discovered: %s, all %v", minVerStr, versions)

	input.MetricsCollector.Set("deckhouse_kubernetes_version", 1, map[string]string{
		"version": minVerStr,
	})
	return nil
}
