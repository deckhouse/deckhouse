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
	"strings"

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
		Mode: 0600,
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

func saveLinstorSosInfo(tarWriter *tar.Writer) error {
	podExtractCmd := []string{"-n", "d8-linstor", "get", "po", "-l", "app=linstor-controller", "-o", "jsonpath='{.items[*].metadata.name}'"}
	output, err := exec.Command("kubectl", podExtractCmd...).Output()
	if err != nil {
		return fmt.Errorf("execute %s command: %v", "kubectl -n d8-linstor get po ...", err)
	}
	podName := strings.TrimSpace(string(output))
	reportGetCmd := []string{"exec", "-n", "d8-linstor", podName, "--", "linstor", "sos-report", "create"}

	reportGenOut, err := exec.Command("kubectl", reportGetCmd...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("execute kubectl %s command with output %s: %v", strings.Join(reportGetCmd, " "), string(reportGenOut), err)
	}
	lines := strings.Split(string(reportGenOut), "\n")
	if len(lines)-2 < 0 {
		return fmt.Errorf("wrong output of command sos-report create: %s", strings.Join(reportGetCmd, " "))
	}
	lastLine := lines[len(lines)-2]
	parts := strings.Split(lastLine, ":")
	if len(parts) != 2 {
		return fmt.Errorf("output doesn't contain file name: %s", lastLine)
	}
	filePathOnPod := strings.TrimSpace(parts[1])
	reportDownloadCmd := []string{"cp", "d8-linstor/" + podName + ":" + filePathOnPod, "linstor-sos.tar.gz"}

	err = exec.Command("kubectl", reportDownloadCmd...).Run()
	if err != nil {
		return fmt.Errorf("error while download sos info report from pod: %v", err)
	}
	file, err := os.Open("linstor-sos.tar.gz")
	if err != nil {
		return fmt.Errorf("error opening linstor-sos.tar.gz file: %v", err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting stat linstor-sos.tar.gz file: %v", err)
	}

	header := &tar.Header{
		Name: "linstor-sos.tar.gz",
		Mode: 0600,
		Size: fileInfo.Size(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header: %v", err)
	}

	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("copy content: %v", err)
	}

	return nil
}

func createTarball(withLinstor bool) *bytes.Buffer {
	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	linstorCommands := []Command{
		{
			File: "linstor-csi-node-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-linstor", "logs", "daemonset.apps/linstor-csi-node", "-c", "linstor-csi-plugin", "--tail", "3000"},
		},
		{
			File: "linstor-node-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-linstor", "logs", "daemonset.apps/linstor-node", "-c", "linstor-satellite", "--tail", "3000"},
		},
		{
			File: "linstor-controller-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-linstor", "logs", "deployment.apps/linstor-controller", "-c", "linstor-controller", "--tail", "3000"},
		},
		{
			File: "linstor-csi-controller-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-linstor", "logs", "deployment.apps/linstor-csi-controller", "--tail", "3000"},
		},
		{
			File: "linstor-drbd-operator-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-linstor", "logs", "deployment.apps/sds-drbd-operator", "--tail", "3000"},
		},
	}

	debugCommands := []Command{
		{
			File: "queue.txt",
			Cmd:  "deckhouse-controller",
			Args: []string{"queue", "list"},
		},
		{
			File: "global-values.json",
			Cmd:  "bash",
			Args: []string{"-c", `deckhouse-controller global values -o json | jq .`},
		},
		{
			File: "deckhouse-enabled-modules.json",
			Cmd:  "bash",
			Args: []string{"-c", "kubectl get modules -o json | jq '.items[]'"},
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
			Args: []string{"get", "machines", "-A", "-o", "json"},
		},
		{
			File: "deckhouse-version.json",
			Cmd:  "bash",
			Args: []string{"-c", "jq -s add <(kubectl -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {\"core.deckhouse.io/edition\",\"core.deckhouse.io/version\"}') <(kubectl -n d8-system get deployment deckhouse -o json | jq -r '.spec.template.spec.containers[] | {image}')"},
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
			Args: []string{"-n", "d8-cloud-instance-manager", "logs", "-l", "app=machine-controller-manager", "--tail", "3000", "-c", "controller"},
		},
		{
			File: "ccm-logs.txt",
			Cmd:  "bash",
			Args: []string{"-c", "kubectl -n $(kubectl get ns -o custom-columns=NAME:metadata.name | grep d8-cloud-provider) logs -l app=cloud-controller-manager --tail=3000"},
		},
		{
			File: "cluster-autoscaler-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-cloud-instance-manager", "logs", "-l", "app=cluster-autoscaler", "--tail", "3000", "-c", "cluster-autoscaler"},
		},
		{
			File: "vpa-admission-controller-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "kube-system", "logs", "-l", "app=vpa-admission-controller", "--tail", "3000", "-c", "admission-controller"},
		},
		{
			File: "vpa-recommender-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "kube-system", "logs", "-l", "app=vpa-recommender", "--tail", "3000", "-c", "recommender"},
		},
		{
			File: "vpa-updater-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "kube-system", "logs", "-l", "app=vpa-updater", "--tail", "3000", "-c", "updater"},
		},
		{
			File: "prometheus-logs.txt",
			Cmd:  "kubectl",
			Args: []string{"-n", "d8-monitoring", "logs", "-l", "prometheus=main", "--tail", "3000", "-c", "prometheus"},
		},
		{
			File: "terraform-check.json",
			Cmd:  "kubectl",
			Args: []string{"exec", "deploy/terraform-state-exporter", "--", "dhctl", "terraform", "check", "--logger-type", "json", "-o", "json"},
		},
		{
			File: "alerts.json",
			Cmd:  "bash",
			Args: []string{"-c", `kubectl get clusteralerts.deckhouse.io -o json | jq '.items[]'`},
		},
	}

	if withLinstor {
		debugCommands = append(debugCommands, linstorCommands...)
		if err := saveLinstorSosInfo(tarWriter); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
		}
	}

	for _, cmd := range debugCommands {
		if err := cmd.Save(tarWriter); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
		}
	}

	return &buf
}

func DefineCollectDebugInfoCommand(kpApp *kingpin.Application) {
	var withLinstor bool
	collectDebug := kpApp.Command("collect-debug-info", "Collect debug info from your cluster.")
	collectDebug.Flag("linstor", "Collect Linstor info").BoolVar(&withLinstor)
	collectDebug.Action(func(c *kingpin.ParseContext) error {
		res := createTarball(withLinstor)
		_, err := io.Copy(os.Stdout, res)
		return err
	})
}
