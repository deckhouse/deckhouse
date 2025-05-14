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

package debug

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"

	"gopkg.in/alecthomas/kingpin.v2"
)

type Command struct {
	Cmd  string
	Args []string

	File string
}

func (c *Command) Save(tarWriter *tar.Writer) error {
	fileContent, err := exec.Command(c.Cmd, c.Args...).Output()
	if err != nil {
		return fmt.Errorf("execute %s %s command: %v", c.Cmd, c.Args, err)
	}

	header := &tar.Header{
		Name: c.File,
		Mode: 0o600,
		Size: int64(len(fileContent)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header: %v", err)
	}

	reader := bytes.NewReader(fileContent)

	if _, err := io.Copy(tarWriter, reader); err != nil {
		return fmt.Errorf("copy content: %v", err)
	}

	return nil
}

func createTarball() *bytes.Buffer {
	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	debugCommands := []Command{
		{
			File: "queue.txt",
			Cmd:  "deckhouse-controller",
			Args: []string{"queue", "list"},
		},
		{
			File: "global-values.json",
			Cmd:  "bash",
			Args: []string{"-c", `deckhouse-controller global values -o json | jq '.internal.modules.kubeRBACProxyCA = "REDACTED" | .modulesImages.registry.dockercfg = "REDACTED"'`},
		},
		{
			File: "deckhouse-enabled-modules.json",
			Cmd:  "bash",
			Args: []string{"-c", "kubectl get modules -o json | jq '.items[]'"},
		},
		{
			File: "deckhouse-maintenance-modules.txt",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl get moduleconfig -ojson | jq -r '.items[] | select(.spec.maintenance == "NoResourceReconciliation") | .metadata.name'`},
		},
		{
			File: "events.json",
			Cmd:  "kubectl",
			Args: []string{"get", "events", "--sort-by=.metadata.creationTimestamp", "-A", "-o", "json"},
		},
		{
			File: "d8-all.json",
			Cmd:  "bash",
			Args: []string{"-c", `for ns in $(kubectl get ns -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}{{"kube-system"}}' -l heritage=deckhouse); do kubectl -n $ns get all -o json; done | jq -s '[.[].items[]]'`},
		},
		{
			File: "node-groups.json",
			Cmd:  "kubectl",
			Args: []string{"get", "nodegroups", "-A", "-o", "json"},
		},
		{
			File: "nodes.json",
			Cmd:  "kubectl",
			Args: []string{"get", "nodes", "-A", "-o", "json"},
		},
		{
			File: "machines.json",
			Cmd:  "kubectl",
			Args: []string{"get", "machines.machine.sapcloud.io", "-A", "-o", "json"},
		},
		{
			File: "instances.json",
			Cmd:  "kubectl",
			Args: []string{"get", "instances.deckhouse.io", "-o", "json"},
		},
		{
			File: "staticinstances.json",
			Cmd:  "kubectl",
			Args: []string{"get", "staticinstances.deckhouse.io", "-o", "json"},
		},
		{
			File: "deckhouse-version.json",
			Cmd:  "bash",
			Args: []string{"-c", "jq -s add <(kubectl -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {\"core.deckhouse.io/edition\",\"core.deckhouse.io/version\"}') <(kubectl -n d8-system get deployment deckhouse -o json | jq -r '.spec.template.spec.containers[] | select(.name == \"deckhouse\") | {image}')"},
		},
		{
			File: "deckhouse-releases.json",
			Cmd:  "kubectl",
			Args: []string{"get", "deckhousereleases", "-o", "json"},
		},
		{
			File: "deckhouse-logs.json",
			Cmd:  "kubectl",
			Args: []string{"logs", "deploy/deckhouse", "--tail", "3000"},
		},
		{
			File: "mcm-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-cloud-instance-manager", "logs", "-l", "app=machine-controller-manager", "--tail=3000", "-c", "controller", "--ignore-errors=true"},
		},
		{
			File: "ccm-logs.txt",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl get modules -o json | jq -r '.items[] | select(.status.phase == "Ready" and (.metadata.name | test("^cloud-provider"))) | "kubectl -n d8-"+.metadata.name+" logs -l app=cloud-controller-manager --tail=3000"' | bash`},
		},
		{
			File: "cluster-autoscaler-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-cloud-instance-manager", "logs", "-l", "app=cluster-autoscaler", "--tail=3000", "-c", "cluster-autoscaler", "--ignore-errors=true"},
		},
		{
			File: "vpa-admission-controller-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "kube-system", "logs", "-l", "app=vpa-admission-controller", "--tail=3000", "-c", "admission-controller", "--ignore-errors=true"},
		},
		{
			File: "vpa-recommender-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "kube-system", "logs", "-l", "app=vpa-recommender", "--tail=3000", "-c", "recommender", "--ignore-errors=true"},
		},
		{
			File: "vpa-updater-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "kube-system", "logs", "-l", "app=vpa-updater", "--tail=3000", "-c", "updater", "--ignore-errors=true"},
		},
		{
			File: "prometheus-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-monitoring", "logs", "-l", "prometheus=main", "--tail=3000", "-c", "prometheus", "--ignore-errors=true"},
		},
		{
			File: "alerts.json",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl get clusteralerts.deckhouse.io -o json | jq '.items[]'`},
		},
		{
			File: "bad-pods.txt",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl get pod -A -owide | grep -Pv '\s+([1-9]+[\d]*)\/\1\s+' | grep -v 'Completed\|Evicted' | grep -E "^(d8-|kube-system)" || true`},
		},
		{
			File: "cluster-authorization-rules.json",
			Cmd:  "kubectl",
			Args: []string{"get", "clusterauthorizationrules", "-o", "json"},
		},
		{
			File: "authorization-rules.json",
			Cmd:  "kubectl",
			Args: []string{"get", "authorizationrules", "-o", "json"},
		},
		{
			File: "module-configs.json",
			Cmd:  "kubectl",
			Args: []string{"get", "moduleconfig", "-o", "json"},
		},
		{
			File: "d8-istio-resources.json",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-istio", "get", "all", "-o", "json"},
		},
		{
			File: "d8-istio-custom-resources.json",
			Cmd:  "bash",
			Args: []string{"-c", `for crd in $(kubectl get crds | grep -E 'istio.io|gateway.networking.k8s.io' | awk '{print $1}'); do echo "Listing resources for CRD: $crd" && kubectl get $crd -A -o json; done`},
		},
		{
			File: "d8-istio-envoy-config.json",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl port-forward daemonset/ingressgateway -n d8-istio 15000:15000 & sleep 5; (curl http://localhost:15000/config_dump?include_eds=true | jq 'del(.configs[6].dynamic_active_secrets)' && kill $!) || { kill $!; exit 1; }`},
		},
		{
			File: "d8-istio-system-logs.txt",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl -n d8-istio logs deployments -l app=istiod`},
		},
		{
			File: "d8-istio-ingress-logs.txt",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl -n d8-istio logs daemonset/ingressgateway || true`},
		},
		{
			File: "d8-istio-users-logs.txt",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl get pods --all-namespaces -o jsonpath='{range .items[?(@.metadata.annotations.istio\.io/rev)]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.spec.containers[*].name}{"\n"}{end}' | awk '/istio-proxy/ {print $0}' | shuf -n 1 | while read namespace pod_name containers; do echo "Collecting logs from istio-proxy in Pod $pod_name (Namespace: $namespace)"; kubectl logs "$pod_name" -n "$namespace" -c istio-proxy; done`},
		},
	}

	for _, cmd := range debugCommands {
		if err := cmd.Save(tarWriter); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
		}
	}

	return &buf
}

func DefineCollectDebugInfoCommand(kpApp *kingpin.Application) {
	collectDebug := kpApp.Command("collect-debug-info", "Collect debug info from your cluster.")
	collectDebug.Action(func(_ *kingpin.ParseContext) error {
		res := createTarball()
		_, err := io.Copy(os.Stdout, res)
		return err
	})
}
