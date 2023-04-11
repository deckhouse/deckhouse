/*
Copyright 2022 Flant JSC

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
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
)

var (
	vectorBinaryPath = "/usr/bin/vector"

	defaultConfig = "/etc/vector/default/defaults.json"
	sampleConfig  = "/opt/vector/vector.json"

	dynamicConfigDir = "/etc/vector/dynamic"
)

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
			log.Fatalf("Can't close fsnotify watcher: %s", err)
		}
	}(watcher)

	err = reloadVectorConfig()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event: ", event)
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					err := reloadVectorConfig()
					if err != nil {
						log.Fatal(err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// TODO: add health checking machinery
				log.Fatalf("inotify watcher returned error: %s", err)
			}
		}
	}()

	// TODO: watch whole directory and react to resolv.conf files changes
	err = watcher.Add(sampleConfig)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: add signal handling
	<-make(chan struct{})
}

func reloadVectorConfig() error {
	tempConfigDir, err := os.MkdirTemp("", "vector-config")
	if err != nil {
		return err
	}

	if ok, err := shouldReload(tempConfigDir); err != nil {
		return err
	} else if !ok {
		return nil
	}

	processes, err := process.Processes()
	if err != nil {
		return err
	}
	for _, p := range processes {
		executable, err := p.Exe()
		if err != nil {
			return err
		}
		if executable == vectorBinaryPath {
			err := p.SendSignal(unix.SIGHUP)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func shouldReload(tempConfigDir string) (bool, error) {
	templatedSampleConfigPath := filepath.Join(tempConfigDir, "vector.json")
	dynamicConfigPath := filepath.Join(dynamicConfigDir, "vector.json")

	sampleConfigContentsBytes, err := os.ReadFile(sampleConfig)
	if err != nil {
		return false, err
	}

	sampleConfigContents := os.ExpandEnv(string(sampleConfigContentsBytes))
	err = os.WriteFile(templatedSampleConfigPath, []byte(sampleConfigContents), 0666)
	if err != nil {
		return false, err
	}

	errOut, err := runVector(fmt.Sprintf("--color never validate --config-json %s --config-json %s", defaultConfig, templatedSampleConfigPath))
	if err != nil {
		return false, fmt.Errorf("skipping config reload, err: %s, vector output: %s", err, errOut)
	}

	oldChecksum, err := getFileChecksum(templatedSampleConfigPath)
	if err != nil {
		return false, err
	}
	newChecksum, err := getFileChecksum(dynamicConfigPath)
	if err != nil {
		return false, err
	}

	return oldChecksum != newChecksum, nil
}

func runVector(args string) (string, error) {
	var errBuffer strings.Builder

	cmd := exec.Command(vectorBinaryPath, strings.Fields(args)...)
	cmd.Env = os.Environ()
	cmd.Stderr = &errBuffer

	return errBuffer.String(), cmd.Run()
}

func getFileChecksum(path string) (string, error) {
	fd, err := os.Open(path)
	defer func(fd *os.File) {
		_ = fd.Close()
	}(fd)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	hash := md5.New()

	_, err = io.Copy(hash, fd)
	if err != nil {
		return "", err
	}

	return string(hash.Sum(nil)), nil
}
