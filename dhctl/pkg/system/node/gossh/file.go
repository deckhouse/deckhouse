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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	ssh "github.com/deckhouse/lib-gossh"
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
		return fmt.Errorf("failed to open local file: %w", err)
	}

	session, err := f.sshClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	if fType != "DIR" {
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

		if err := CopyFile(ctx, localFile, remotePath, "0755", session); err != nil {
			return fmt.Errorf("failed to copy file to remote host: %w", err)
		}
	} else {
		err = session.Run("mkdir -p " + remotePath)
		if err != nil {
			return err
		}
		files, err := os.ReadDir(srcPath)
		if err != nil {
			return fmt.Errorf("could not read directory: %w", err)
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
		localFile, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("failed to open local file: %w", err)
		}
		defer localFile.Close()
		if err := CopyFromRemote(ctx, localFile, remotePath, f.sshClient); err != nil {
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

type PassThru func(r io.Reader, total int64) io.Reader

func CopyFile(
	ctx context.Context,
	fileReader io.Reader,
	remotePath string,
	permissions string,
	session *ssh.Session,
) error {
	contentsBytes, err := io.ReadAll(fileReader)
	if err != nil {
		return fmt.Errorf("failed to read all data from reader: %w", err)
	}
	r := bytes.NewReader(contentsBytes)
	size := int64(len(contentsBytes))

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	w, err := session.StdinPipe()
	if err != nil {
		return err
	}
	defer w.Close()

	filename := path.Base(remotePath)

	// Start the command first and get confirmation that it has been started
	// before sending anything through the pipes.
	err = session.Start(fmt.Sprintf("%s -qt %q", "scp", remotePath))
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	errCh := make(chan error, 2)

	// SCP protocol and file sending
	go func() {
		defer wg.Done()
		defer w.Close()

		_, err = fmt.Fprintln(w, "C"+permissions, size, filename)
		if err != nil {
			errCh <- err
			return
		}

		if err = checkResponse(stdout); err != nil {
			errCh <- err
			return
		}

		_, err = io.Copy(w, r)
		if err != nil {
			errCh <- err
			return
		}

		_, err = fmt.Fprint(w, "\x00")
		if err != nil {
			errCh <- err
			return
		}

		if err = checkResponse(stdout); err != nil {
			errCh <- err
			return
		}
	}()

	// Wait for the process to exit
	go func() {
		defer wg.Done()
		err := session.Wait()
		if err != nil {
			errCh <- err
			return
		}
	}()

	// Wait for one of the conditions (error/timeout/completion) to occur
	if err := wait(&wg, ctx); err != nil {
		return err
	}

	close(errCh)

	// Collect any errors from the error channel
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func checkResponse(r io.Reader) error {
	_, err := scp.ParseResponse(r, nil)
	if err != nil {
		return err
	}

	return nil

}

func wait(wg *sync.WaitGroup, ctx context.Context) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()

	select {
	case <-c:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func CopyFromRemote(ctx context.Context, file *os.File, remotePath string, sshClient *ssh.Client) error {

	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("Error creating ssh session in copy from remote: %v", err)
	}
	defer session.Close()

	wg := sync.WaitGroup{}
	errCh := make(chan error, 4)

	wg.Add(1)
	go func() {
		var err error

		defer func() {
			// NOTE: this might send an already sent error another time, but since we only receive one, this is fine. On the "happy-path" of this function, the error will be `nil` therefore completing the "err<-errCh" at the bottom of the function.
			errCh <- err
			// We must unblock the go routine first as we block on reading the channel later
			wg.Done()

		}()

		r, err := session.StdoutPipe()
		if err != nil {
			errCh <- err
			return
		}

		in, err := session.StdinPipe()
		if err != nil {
			errCh <- err
			return
		}
		defer in.Close()

		err = session.Start(fmt.Sprintf("%s -f %q", "scp", remotePath))
		if err != nil {
			errCh <- err
			return
		}

		err = scp.Ack(in)
		if err != nil {
			errCh <- err
			return
		}

		fileInfo, err := scp.ParseResponse(r, in)
		if err != nil {
			errCh <- err
			return
		}

		err = scp.Ack(in)
		if err != nil {
			errCh <- err
			return
		}

		_, err = scp.CopyN(file, r, fileInfo.Size)
		if err != nil {
			errCh <- err
			return
		}

		err = scp.Ack(in)
		if err != nil {
			errCh <- err
			return
		}

		err = session.Wait()
		if err != nil {
			errCh <- err
			return
		}
	}()

	if err := wait(&wg, ctx); err != nil {
		return err
	}

	finalErr := <-errCh
	close(errCh)
	return finalErr
}
