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
	"fmt"
	"log"
	"os"
	"strings"

	cp "github.com/otiai10/copy"
	"golang.org/x/sys/unix"
)

func main() {
	gfPathsLogs := os.Getenv("GF_PATHS_LOGS")
	gfPathsPlugins := os.Getenv("GF_PATHS_PLUGINS")
	gfPathsProvisioning := os.Getenv("GF_PATHS_PROVISIONING")
	bundledPluginsPath := os.Getenv("BUNDLED_PLUGINS_PATH")
	gfInstallPlugins := os.Getenv("GF_INSTALL_PLUGINS")

	_, err := os.Stat(gfPathsPlugins)
	if os.IsNotExist(err) {
		err := os.MkdirAll(gfPathsPlugins, 0600)
		if err != nil {
			log.Fatalf("create plugins folder: %v", err)
		}
	}
	if err := unix.Access(gfPathsPlugins, unix.W_OK); err != nil {
		log.Fatalf("GF_PATHS_PLUGINS='%s' is not writable.\nYou may have issues with file permissions, more information here: http://docs.grafana.org/installation/docker/#migrate-to-v51-or-later", gfPathsPlugins)
	}

	if bundledPluginsPath != "" && gfInstallPlugins != "" && gfPathsPlugins != bundledPluginsPath {
		_, err = os.Stat(bundledPluginsPath)
		if err == nil {
			err := cp.Copy(bundledPluginsPath, gfPathsPlugins)
			if err != nil {
				log.Fatalf("copy plugins: %v", err)
			}
		}
	}

	gfPathsConfig := os.Getenv("GF_PATHS_CONFIG")
	if err := unix.Access(gfPathsConfig, unix.R_OK); err != nil {
		log.Fatalf("GF_PATHS_CONFIG='%s' is not readable.\nYou may have issues with file permissions, more information here: http://docs.grafana.org/installation/docker/#migrate-to-v51-or-later", gfPathsConfig)
	}

	gfPathsData := os.Getenv("GF_PATHS_DATA")
	if err := unix.Access(gfPathsData, unix.W_OK); err != nil {
		log.Fatalf("GF_PATHS_DATA='%s' is not writable.\nYou may have issues with file permissions, more information here: http://docs.grafana.org/installation/docker/#migrate-to-v51-or-later", gfPathsData)
	}

	gfPathsHome := os.Getenv("GF_PATHS_HOME")
	if err := unix.Access(gfPathsHome, unix.R_OK); err != nil {
		log.Fatalf("GF_PATHS_HOME='%s' is not readable.\nYou may have issues with file permissions, more information here: http://docs.grafana.org/installation/docker/#migrate-to-v51-or-later", gfPathsHome)
	}

	gfAWSProfile, ok := os.LookupEnv("GF_AWS_PROFILES")
	if ok && gfAWSProfile != "" {

		credentialsFile, err := os.OpenFile(
			fmt.Sprintf("%s/.aws/credentials", gfPathsHome),
			os.O_RDWR,
			0600,
		)
		if err != nil {
			log.Fatalf("open credentials file: %v", err)
		}
		defer credentialsFile.Close()
		credentialsFile.Truncate(0)
		credentialsFile.Seek(0, 0)

		builder := strings.Builder{}
		for _, profile := range strings.Split(gfAWSProfile, " ") {
			accessKeyVarname := os.Getenv(
				fmt.Sprintf("GF_AWS_%s_ACCESS_KEY_ID", strings.ToUpper(profile)),
			)
			secretKeyVarname := os.Getenv(
				fmt.Sprintf("GF_AWS_%s_SECRET_ACCESS_KEY", strings.ToUpper(profile)),
			)
			regionVarname := os.Getenv(
				fmt.Sprintf("GF_AWS_%s_REGION", strings.ToUpper(profile)),
			)
			if accessKeyVarname == "" || secretKeyVarname == "" {
				continue
			}

			builder.Reset()
			builder.WriteString("[")
			builder.WriteString(profile)
			builder.WriteString("]")
			builder.WriteString("\n")
			builder.WriteString("aws_access_key_id = ")
			builder.WriteString(accessKeyVarname)
			builder.WriteString("\n")
			builder.WriteString("aws_secret_access_key = ")
			builder.WriteString(secretKeyVarname)
			builder.WriteString("\n")

			if regionVarname != "" {
				builder.WriteString("region = ")
				builder.WriteString(regionVarname)
				builder.WriteString("\n")
			}

			_, err := credentialsFile.WriteString(builder.String())
			if err != nil {
				log.Fatalf("write to credentials file: %v", err)
			}
		}
	}

	grafanaArgs := []string{
		"grafana",
		fmt.Sprintf("--homepath=%s", gfPathsHome),
		fmt.Sprintf("--config=%s", gfPathsConfig),
		"--packaging=docker",
		"cfg:default.log.mode=console",
		fmt.Sprintf("cfg:default.paths.data=%s", gfPathsData),
		fmt.Sprintf("cfg:default.paths.logs=%s", gfPathsLogs),
		fmt.Sprintf("cfg:default.paths.plugins=%s", gfPathsPlugins),
		fmt.Sprintf("cfg:default.paths.provisioning=%s", gfPathsProvisioning),
	}

	err = unix.Exec("/usr/share/grafana/bin/grafana-server", grafanaArgs, os.Environ())
	if err != nil {
		log.Fatalf("exec grafana: %v", err)
	}
}
