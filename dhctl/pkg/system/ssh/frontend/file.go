// Copyright 2021 Flant JSC
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

package frontend

import (
	"fmt"
	"os"
	"path/filepath"

	uuid "gopkg.in/satori/go.uuid.v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type File struct {
	Session *session.Session
}

func NewFile(sess *session.Session) *File {
	return &File{Session: sess}
}

func (f *File) Upload(srcPath, remotePath string) error {
	fType, err := CheckLocalPath(srcPath)
	if err != nil {
		return err
	}
	scp := cmd.NewSCP(f.Session)
	if fType == "DIR" {
		scp.WithRecursive(true)
	}
	scp.WithSrc(srcPath).
		WithRemoteDst(remotePath).
		SCP().
		CaptureStdout(nil).
		CaptureStderr(nil)
	err = scp.Run()
	if err != nil {
		return fmt.Errorf(
			"upload file '%s': %w\n%s\nstderr: %s",
			srcPath,
			err,
			string(scp.StdoutBytes()),
			string(scp.StderrBytes()),
		)
	}

	return nil
}

// UploadBytes creates a tmp file and upload it to remote dstPath
func (f *File) UploadBytes(data []byte, remotePath string) error {
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

	scp := cmd.NewSCP(f.Session).
		WithSrc(srcPath).
		WithRemoteDst(remotePath).
		SCP().
		CaptureStderr(nil).
		CaptureStdout(nil)
	err = scp.Run()
	if err != nil {
		return fmt.Errorf(
			"upload file '%s': %w\n%s\nstderr: %s",
			remotePath,
			err,
			string(scp.StdoutBytes()),
			string(scp.StderrBytes()),
		)
	}

	if len(scp.StdoutBytes()) > 0 {
		log.InfoF("Upload file: %s", string(scp.StdoutBytes()))
	}
	return nil
}

func (f *File) Download(remotePath, dstPath string) error {
	scp := cmd.NewSCP(f.Session)
	scp.WithRecursive(true)
	scpCmd := scp.WithRemoteSrc(remotePath).WithDst(dstPath).SCP()
	log.DebugF("run scp: %s\n", scpCmd.Cmd().String())

	stdout, err := scpCmd.Cmd().CombinedOutput()
	if err != nil {
		return fmt.Errorf("download file '%s': %w", remotePath, err)
	}

	if len(stdout) > 0 {
		log.InfoF("Download file: %s", string(stdout))
	}
	return nil
}

// Download remote file and returns its content as an array of bytes.
func (f *File) DownloadBytes(remotePath string) ([]byte, error) {
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

	scp := cmd.NewSCP(f.Session)
	scpCmd := scp.WithRemoteSrc(remotePath).WithDst(dstPath).SCP()
	log.DebugF("run scp: %s\n", scpCmd.Cmd().String())

	stdout, err := scpCmd.Cmd().CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("download file '%s': %w", remotePath, err)
	}

	if len(stdout) > 0 {
		log.InfoF("Download file: %s", string(stdout))
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		return nil, fmt.Errorf("reading tmp file '%s': %w", dstPath, err)
	}

	return data, nil
}

func CreateEmptyTmpFile() (string, error) {
	tmpPath := filepath.Join(
		os.TempDir(),
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
