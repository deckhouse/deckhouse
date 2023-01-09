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

package library

import (
	"fmt"
	"os"
	"path/filepath"
)

type MergeStrategy int

const (
	ThrowError MergeStrategy = iota
	StashInTemp
)

type MergeTarget struct {
	Strategy MergeStrategy
	NewName  string
}

type MergeTargets map[string]MergeTarget

type MergeConf struct {
	Targets MergeTargets
	TempDir string
}

func MergeEditions(conf MergeConf) error {
	if isGithub() {
		return nil
	}

	if conf.TempDir != "" {
		err := os.MkdirAll(filepath.Join(os.TempDir(), conf.TempDir), 0755)
		if err != nil {
			return err
		}
	}

	for oldName, target := range conf.Targets {
		exists, err := exists(target.NewName)
		if err != nil {
			return fmt.Errorf("error checking if destination exists: \"%s\"", target.NewName)
		}
		if exists {
			switch target.Strategy {
			case ThrowError:
				return fmt.Errorf("destination already exists - unable to merge path \"%s\" to \"%s\"", oldName, target.NewName)
			case StashInTemp:
				err := os.Rename(oldName, filepath.Join(os.TempDir(), conf.TempDir, filepath.Base(oldName)))
				if err != nil {
					return err
				}
			}
		}
		err = os.Symlink(oldName, target.NewName)
		if err != nil {
			return fmt.Errorf("error creating symlink from path \"%s\" to \"%s\"", oldName, target.NewName)
		}
	}

	return nil
}

func RestoreEditions(conf MergeConf) error {
	if isGithub() {
		return nil
	}

	for oldName, target := range conf.Targets {
		err := os.Remove(target.NewName)
		if err != nil {
			return err
		}
		switch target.Strategy {
		case ThrowError:
			// Do nothing
		case StashInTemp:
			err := os.Rename(filepath.Join(os.TempDir(), conf.TempDir, filepath.Base(oldName)), oldName)
			if err != nil {
				return err
			}
		}
	}

	if conf.TempDir != "" {
		err := os.RemoveAll(filepath.Join(os.TempDir(), conf.TempDir))
		if err != nil {
			return err
		}
	}

	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func isGithub() bool {
	_, isGithub := os.LookupEnv("GITHUB_ACTIONS")
	return isGithub
}
