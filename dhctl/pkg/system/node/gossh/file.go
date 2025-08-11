// Copyright 2025 Flant JSC
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

package gossh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"golang.org/x/crypto/ssh"
	uuid "gopkg.in/satori/go.uuid.v1"
)

type SSHFile struct {
	sshClient *ssh.Client
}

func NewSSHFile(client *ssh.Client) *SSHFile {
	return &SSHFile{sshClient: client}
}

func (f *SSHFile) Upload(ctx context.Context, srcPath, remotePath string) error {
	fType, err := CheckLocalPath(srcPath)
	if err != nil {
		return err
	}

	if fType != "DIR" {
		scpClient, err := scp.NewClientBySSH(f.sshClient)
		if err != nil {
			return err
		}
		defer scpClient.Close()
		localFile, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open local file: %w", err)
		}
		defer localFile.Close()

		rType, err := getRemoteFileStat(f.sshClient, remotePath)
		if err != nil {
			if !strings.ContainsAny(err.Error(), "No such file or directory") {
				return err
			}
		}
		if rType == "DIR" {
			remotePath = remotePath + "/" + filepath.Base(srcPath)
		}
		log.DebugF("starting upload local %s to remote %s\n", srcPath, remotePath)

		if err := scpClient.CopyFile(ctx, localFile, remotePath, "0755"); err != nil {
			return fmt.Errorf("failed to copy file to remote host: %w", err)
		}
	} else {
		session, err := f.sshClient.NewSession()
		if err != nil {
			return err
		}
		defer session.Close()

		err = session.Run("mkdir -p " + remotePath)
		if err != nil {
			return err
		}
		scpClient, err := scp.NewClientBySSH(f.sshClient)
		if err != nil {
			return err
		}
		defer scpClient.Close()
		files, err := os.ReadDir(srcPath)
		if err != nil {
			return err
		}
		for _, file := range files {
			err = f.Upload(ctx, srcPath+"/"+file.Name(), remotePath+"/"+file.Name())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// UploadBytes creates a tmp file and upload it to remote dstPath
func (f *SSHFile) UploadBytes(ctx context.Context, data []byte, remotePath string) error {
	srcPath, err := CreateEmptyTmpFile()
	if err != nil {
		return fmt.Errorf("create source tmp file: %v", err)
	}
	defer func() {
		err := os.Remove(srcPath)
		if err != nil {
			log.ErrorF("Error: cannot remove tmp file '%s': %v\n", srcPath, err)
		}
	}()

	err = os.WriteFile(srcPath, data, 0o600)
	if err != nil {
		return fmt.Errorf("write data to tmp file: %w", err)
	}

	err = f.Upload(ctx, srcPath, remotePath)
	return err
}

func (f *SSHFile) Download(ctx context.Context, remotePath, dstPath string) error {
	fType, err := getRemoteFileStat(f.sshClient, remotePath)
	if err != nil {
		return err
	}

	if fType != "DIR" {
		// regular file logic
		scpClient, err := scp.NewClientBySSH(f.sshClient)
		if err != nil {
			return err
		}
		defer scpClient.Close()
		localFile, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("failed to open local file: %w", err)
		}
		defer localFile.Close()
		if err := scpClient.CopyFromRemote(ctx, localFile, remotePath); err != nil {
			return fmt.Errorf("failed to copy file to remote host: %w", err)
		}
	} else {
		// recursive copy logic
		filesString, err := getRemoteFilesList(f.sshClient, remotePath)
		if err != nil {
			return err
		}

		if filepath.Base(dstPath) != filepath.Base(remotePath) {
			dstPath = dstPath + "/" + filepath.Base(remotePath)
		}

		err = os.MkdirAll(dstPath, os.ModePerm)
		if err != nil {
			return err
		}

		re := regexp.MustCompile(`\s+`)
		files := re.Split(filesString, -1)
		for _, file := range files {
			f.Download(ctx, remotePath+"/"+file, dstPath+"/"+file)
		}
	}

	return nil
}

// Download remote file and returns its content as an array of bytes.
func (f *SSHFile) DownloadBytes(ctx context.Context, remotePath string) ([]byte, error) {
	dstPath, err := CreateEmptyTmpFile()
	if err != nil {
		return nil, fmt.Errorf("create target tmp file: %v", err)
	}
	defer func() {
		err := os.Remove(dstPath)
		if err != nil {
			log.InfoF("Error: cannot remove tmp file '%s': %v\n", dstPath, err)
		}
	}()

	err = f.Download(ctx, remotePath, dstPath)
	if err != nil {
		return nil, fmt.Errorf("download target tmp file: %v", err)
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		return nil, fmt.Errorf("reading tmp file '%s': %w", dstPath, err)
	}

	return data, nil
}

func getRemoteFileStat(client *ssh.Client, remoteFilePath string) (string, error) {
	if remoteFilePath == "." {
		return "DIR", nil
	}

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	command := fmt.Sprint("LC_ALL=en_US.utf8 stat -c %F " + remoteFilePath)
	output, err := session.CombinedOutput(command)

	log.DebugF("remote path %s is %s\n", remoteFilePath, output)

	if strings.TrimSpace(string(output)) == "directory" {
		return "DIR", nil
	}

	if strings.TrimSpace(string(output)) == "regular file" {
		return "FILE", nil
	}

	return "", err
}

func getRemoteFilesList(client *ssh.Client, remoteFilePath string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	command := fmt.Sprint("ls " + remoteFilePath)
	output, err := session.CombinedOutput(command)

	return strings.TrimSpace(string(output)), err
}

func CreateEmptyTmpFile() (string, error) {
	tmpPath := filepath.Join(
		app.TmpDirName,
		fmt.Sprintf("dhctl-scp-%d-%s.tmp", os.Getpid(), uuid.NewV4().String()),
	)

	file, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}

	_ = file.Close()
	return tmpPath, nil
}

// CheckLocalPath see if file exists and determine if it is a directory. Error is returned if file is not exists.
func CheckLocalPath(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.Mode().IsDir() {
		return "DIR", nil
	}
	if fi.Mode().IsRegular() {
		return "FILE", nil
	}
	return "", fmt.Errorf("path '%s' is not a directory or file", path)
}
