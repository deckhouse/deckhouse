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
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

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
		err := os.MkdirAll(gfPathsPlugins, 0660)
		if err != nil {
			log.Fatalf("create plugins folder: %v", err)
		}
	}
	if err := unix.Access(gfPathsPlugins, unix.W_OK); err != nil {
		log.Fatalf("GF_PATHS_PLUGINS='%s' is not writable.\nYou may have issues with file permissions, more information here: http://docs.grafana.org/installation/docker/#migrate-to-v51-or-later", gfPathsPlugins)
	}

	if bundledPluginsPath != "" && gfInstallPlugins != "" && gfPathsPlugins != bundledPluginsPath {
		fstatDest, err := os.Stat(gfPathsPlugins)
		if err != nil {
			log.Fatalf("file info error: path: %s err: %v", gfPathsPlugins, err)
		}

		_, err = os.Stat(bundledPluginsPath)
		if err == nil {
			opt := cp.Options{
				OnError: func(src, dest string, err error) error {
					if strings.Contains(errors.Join(err, errors.New("")).Error(), "chmod /etc/grafana/plugins: operation not permitted") {
						return nil
					}
					return err
				},
				// add permissions from destination folder
				PermissionControl: cp.AddPermission(fstatDest.Mode()),
			}
			err := cp.Copy(bundledPluginsPath, gfPathsPlugins, opt)
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

	err = convertEnv()
	if err != nil {
		log.Fatalf("convert env: %v", err)
	}

	os.Setenv("HOME", gfPathsHome)

	err = installPlugins(gfInstallPlugins, gfPathsPlugins)
	if err != nil {
		log.Fatalf("install plugins: %v", err)
	}

	grafanaArgs := []string{
		"grafana",
		"server",
		fmt.Sprintf("--homepath=%s", gfPathsHome),
		fmt.Sprintf("--config=%s", gfPathsConfig),
		"--packaging=docker",
		"cfg:default.log.mode=console",
		fmt.Sprintf("cfg:default.paths.data=%s", gfPathsData),
		fmt.Sprintf("cfg:default.paths.logs=%s", gfPathsLogs),
		fmt.Sprintf("cfg:default.paths.plugins=%s", gfPathsPlugins),
		fmt.Sprintf("cfg:default.paths.provisioning=%s", gfPathsProvisioning),
	}

	grafanaBin := "/usr/share/grafana/bin/grafana"

	err = syscall.Exec(grafanaBin, grafanaArgs, os.Environ())
	if err != nil {
		log.Fatalf("exec %s: %v", grafanaBin, err)
	}
}

const (
	paramPrefix = "GF_"
	paramSuffix = "__FILE"
)

// convertEnv Convert all environment variables with names ending in __FILE into the content of
// the file that they point at and use the name without the trailing __FILE.
// This can be used to carry in Docker secrets.
func convertEnv() error {
	for _, param := range os.Environ() {
		if !strings.HasPrefix(param, paramPrefix) {
			continue
		}
		splitedParam := strings.Split(param, "=")
		if !strings.HasSuffix(splitedParam[0], paramSuffix) {
			continue
		}
		newParamName := strings.TrimRight(splitedParam[0], paramSuffix)
		_, ok := os.LookupEnv(newParamName)
		if ok {
			return fmt.Errorf("error: both %s and %s are set (but are exclusive)", newParamName, splitedParam[0])
		}

		content, err := os.ReadFile(splitedParam[1])
		if err != nil {
			log.Fatalf("open file: %v", err)
		}

		os.Setenv(newParamName, string(content))
		os.Unsetenv(splitedParam[0])
	}
	return nil
}

func installPlugins(gfInstallPlugins, gfPathsPlugins string) error {
	if gfInstallPlugins == "" {
		return nil
	}

	for _, plugin := range strings.Split(gfInstallPlugins, ",") {

		if strings.Contains(plugin, ";") {
			part := strings.Split(plugin, ";")
			cmd := exec.Command(
				"grafana-cli",
				"--pluginUrl",
				part[0],
				"--pluginsDir",
				gfPathsPlugins,
				"plugins",
				"install",
				part[1],
			)

			if stdout, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%v | %v", string(stdout), err)
			}
			continue
		}
		cmd := exec.Command(
			"grafana-cli",
			"--pluginsDir",
			gfPathsPlugins,
			"plugins",
			"install",
			plugin,
		)
		if stdout, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v | %v", string(stdout), err)
		}
	}
	return nil
}
