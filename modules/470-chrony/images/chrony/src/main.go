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
	"bytes"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

const (
	chronyConfPath     = "/etc/chrony/chrony.conf"
	chronyConfTplPath  = "/etc/chrony/chrony.conf.tpl"
	chronydPidFilePath = "/var/run/chrony/chronyd.pid"
	chronydPath        = "/opt/chrony-static/sbin/chronyd"
)

type ChronyConfigTemplateData struct {
	NTPRole              string
	NTPServers           []string
	ChronyMastersService string
	HostIP               string
}

func main() {
	ntpServers := os.Getenv("NTP_SERVERS")

	var ntpServersList []string
	if ntpServers != "" {
		ntpServersList = strings.Split(os.Getenv("NTP_SERVERS"), " ")
	}

	configTemplateData := ChronyConfigTemplateData{
		NTPRole:              os.Getenv("NTP_ROLE"),
		NTPServers:           ntpServersList,
		ChronyMastersService: os.Getenv("CHRONY_MASTERS_SERVICE"),
		HostIP:               os.Getenv("HOST_IP"),
	}

	configBuffer := &bytes.Buffer{}

	err := template.Must(template.New(path.Base(chronyConfTplPath)).ParseFiles(chronyConfTplPath)).Execute(configBuffer, configTemplateData)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to execute %s template", chronyConfTplPath))
	}

	err = os.WriteFile(chronyConfPath, configBuffer.Bytes(), 0600)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to write %s file", chronyConfPath))
	}

	err = os.Remove(chronydPidFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(errors.Wrapf(err, "failed to remove %s file", chronydPidFilePath))
	}

	cmd := exec.Command(chronydPath, "-4", "-d", "-s", "-f", chronyConfPath)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to exec %s", chronydPath))
	}
}
