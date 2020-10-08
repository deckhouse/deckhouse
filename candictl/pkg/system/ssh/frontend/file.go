package frontend

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	uuid "gopkg.in/satori/go.uuid.v1"

	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh/cmd"
	"flant/candictl/pkg/system/ssh/session"
)

type File struct {
	Session *session.Session
}

func NewFile(sess *session.Session) *File {
	return &File{Session: sess}
}

func (f *File) Upload(srcPath string, remotePath string) error {
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
		return fmt.Errorf("upload file '%s': %v\n%s\nstderr: %s", srcPath, err, string(scp.StdoutBytes()), string(scp.StderrBytes()))
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

	err = ioutil.WriteFile(srcPath, data, 0600)
	if err != nil {
		return fmt.Errorf("write data to tmp file: %v", err)
	}

	scp := cmd.NewSCP(f.Session).
		WithSrc(srcPath).
		WithRemoteDst(remotePath).
		SCP().
		CaptureStderr(nil).
		CaptureStdout(nil)
	err = scp.Run()
	if err != nil {
		return fmt.Errorf("upload file '%s': %v\n%s\nstderr: %s", remotePath, err, string(scp.StdoutBytes()), string(scp.StderrBytes()))
	}

	if len(scp.StdoutBytes()) > 0 {
		log.InfoF("Upload file: %s", string(scp.StdoutBytes()))
	}
	return nil
}

func (f *File) Download(remotePath string, dstPath string) error {
	scp := cmd.NewSCP(f.Session)
	scp.WithRecursive(true)
	scpCmd := scp.WithRemoteSrc(remotePath).WithDst(dstPath).SCP()
	log.DebugF("run scp: %s\n", scpCmd.Cmd().String())

	stdout, err := scpCmd.Cmd().CombinedOutput()
	if err != nil {
		return fmt.Errorf("download file '%s': %v", remotePath, err)
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
		return nil, fmt.Errorf("download file '%s': %v", remotePath, err)
	}

	if len(stdout) > 0 {
		log.InfoF("Download file: %s", string(stdout))
	}

	data, err := ioutil.ReadFile(dstPath)
	if err != nil {
		return nil, fmt.Errorf("reading tmp file '%s': %v", dstPath, err)
	}

	return data, nil
}

func CreateEmptyTmpFile() (string, error) {
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("candictl-scp-%d-%s.tmp", os.Getpid(), uuid.NewV4().String()))

	file, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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
