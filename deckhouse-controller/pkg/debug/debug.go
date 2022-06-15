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
			Cmd:  "deckhouse-controller",
			Args: []string{"global", "values", "-o", "json"},
		},
		{
			File: "deckhouse-enabled-modules.json",
			Cmd:  "deckhouse-controller",
			Args: []string{"module", "list", "-o", "json"},
		},
		{
			File: "events.json",
			Cmd:  "kubectl",
			Args: []string{"get", "events", "-A", "-o", "json"},
		},
		{
			File: "all.json",
			Cmd:  "bash",
			Args: []string{"-c", `for ns in $(kubectl get ns -o jsonpath='{$.items[*].metadata.name}' -l heritage=deckhouse | sed 's/\ /\n/g'); do kubectl -n $ns get all -o json; done | jq -s '[.[].items[]]'`},
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
			File: "terraform-check.json",
			Cmd:  "kubectl",
			Args: []string{"exec", "deploy/terraform-state-exporter", "--", "dhctl", "terraform", "check", "--logger-type", "json", "-o", "json"},
		},
		{
			File: "alerts.json",
			Cmd:  "bash",
			Args: []string{"-c", `curl -kf -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" "https://prometheus.d8-monitoring:9090/api/v1/rules?type=alert" | jq -rc '.data.groups[].rules[] | select(.state == "firing")'`},
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
	collectDebug.Action(func(c *kingpin.ParseContext) error {
		res := createTarball()
		_, err := io.Copy(os.Stdout, res)
		return err
	})
}
