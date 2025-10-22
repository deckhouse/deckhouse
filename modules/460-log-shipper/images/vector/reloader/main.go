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
	"crypto/md5"
	"encoding/hex"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vector/internal"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
)

const (
	vectorBinPath = "/usr/bin/vector"

	defaultConfigPath = "/etc/vector/default/defaults.json"

	sampleConfigPath  = "/opt/vector/vector.json"
	dynamicConfigPath = "/etc/vector/dynamic/vector.json"
)

// pkill -P vector SIGHUP
func sendReloadSignal() error {
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	for _, p := range processes {
		cmdline, err := p.Cmdline()
		if err != nil {
			return err
		}

		if strings.Contains(cmdline, vectorBinPath) {
			err := p.SendSignal(unix.SIGHUP)
			if err != nil {
				return err
			}
			// There can be more than one processes of vector running
			// because of 'vector top' and 'vector vrl' debug commands
			break
		}
	}
	return nil
}

func reloadOnce() {
	log.Println("start reloading Vector config")

	sampleConfig := LoadConfig(sampleConfigPath)
	dynamicConfig := LoadConfig(dynamicConfigPath)

	if compareConfigs(dynamicConfig, sampleConfig) {
		log.Println("configs are equal, doing nothing")
		return
	}

	if err := sampleConfig.Validate(); err != nil {
		log.Println("invalid config, skip running")
		return
	}

	if err := sampleConfig.SaveTo(dynamicConfigPath); err != nil {
		log.Println(err)
		return
	}

	cleanLocks()
	if err := sendReloadSignal(); err != nil {
		log.Println(err)
		return
	}

	log.Println("Vector config has been reloaded")
}

// Vector sets lock for each buffer, but doesn't clean them if the process was killed.
// If locks are left, after restart Vector will not be able to load the config.
func cleanLocks() {
	_ = filepath.Walk("/vector-data/", func(path string, f os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if f == nil {
			return nil
		}

		if filepath.Ext(f.Name()) == "buffer.lock" {
			if err := os.Remove(path); err != nil {
				log.Println(err.Error())
			} else {
				log.Printf("lock file %s has been successfully removed\n", path)
			}
		}
		return nil
	})
}

// Config represents Vector configuration file.
// https://vector.dev/docs/reference/configuration/
type Config struct {
	content []byte
	path    string
}

func LoadConfig(path string) *Config {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return &Config{content: content, path: path}
}

func (c *Config) HashSum() string {
	hash := md5.Sum(c.content)
	return hex.EncodeToString(hash[:])
}

func (c *Config) SaveTo(path string) error {
	return os.WriteFile(path, c.content, 0666)
}

// Validate executes 'vector validate' command.
func (c *Config) Validate() error {
	cmd := exec.Command(
		vectorBinPath,
		"--color", "never",
		"validate",
		"--config-json", defaultConfigPath,
		"--config-json", c.path,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// compareConfigs compares md5 hash of config files and prints diff if so.
func compareConfigs(c1, c2 *Config) bool {
	res := c1.HashSum() == c2.HashSum()
	if res {
		return true
	}
	diff := internal.Diff(c1.path, c1.content, c2.path, c2.content)
	log.Println(string(diff))

	return false
}

func main() {
	cleanLocks()

	if err := LoadConfig(sampleConfigPath).SaveTo(dynamicConfigPath); err != nil {
		log.Fatal(err)
		return
	}
	if err := sendReloadSignal(); err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("initial Vector config has been applied")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(sampleConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("start watching Vector config changes")
	for {
		select {
		case event := <-watcher.Events:
			if event.Op == fsnotify.Remove {
				// k8s configmaps use symlinks,
				// old file is deleted and a new link with the same name is created
				_ = watcher.Remove(event.Name)
				if err := watcher.Add(event.Name); err != nil {
					log.Fatal(err)
				}
				switch event.Name {
				case sampleConfigPath:
					reloadOnce()
				}
			}

		case err := <-watcher.Errors:
			log.Printf("watch files error: %s\n", err)
		}
	}
}
