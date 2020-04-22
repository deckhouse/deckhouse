package frontend

import (
	"flant/deckhouse-candi/pkg/ssh/cmd"
	"flant/deckhouse-candi/pkg/ssh/session"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	uuid "gopkg.in/satori/go.uuid.v1"

	"flant/deckhouse-candi/pkg/app"
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
	scp := cmd.NewScp(f.Session)
	if fType == "DIR" {
		scp.WithRecursive(true)
	}
	scpCmd := scp.WithSrc(srcPath).WithRemoteDst(remotePath).Cmd()
	app.Debugf("run scp: %s\n", scpCmd.String())
	//app.Debugf("run scp: %#v\n", scpCmd)
	err = scpCmd.Run()
	if err != nil {
		return fmt.Errorf("upload file '%s': %v", srcPath, err)
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
			fmt.Printf("Error: cannot remove tmp file '%s': %v\n", srcPath, err)
		}
	}()

	err = ioutil.WriteFile(srcPath, data, 0644)
	if err != nil {
		return fmt.Errorf("write data to tmp file: %v", err)
	}

	scp := cmd.NewScp(f.Session)
	scpCmd := scp.WithSrc(srcPath).WithRemoteDst(remotePath).Cmd()
	app.Debugf("run scp: %s\n", scpCmd.String())
	//app.Debugf("run scp: %#v\n", scpCmd)
	err = scpCmd.Run()
	if err != nil {
		return fmt.Errorf("upload file '%s': %v", remotePath, err)
	}

	return nil
}

func (f *File) Download(remotePath string, dstPath string) error {
	scp := cmd.NewScp(f.Session)
	scp.WithRecursive(true)
	scpCmd := scp.WithRemoteSrc(remotePath).WithDst(dstPath).Cmd()
	app.Debugf("run scp: %s\n", scpCmd.String())
	//app.Debugf("run scp: %#v\n", scpCmd)
	err := scpCmd.Run()
	if err != nil {
		return fmt.Errorf("download file '%s': %v", remotePath, err)
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
			fmt.Printf("Error: cannot remove tmp file '%s': %v\n", dstPath, err)
		}
	}()

	scp := cmd.NewScp(f.Session)
	scpCmd := scp.WithRemoteSrc(remotePath).WithDst(dstPath).Cmd()
	app.Debugf("run scp: %s\n", scpCmd.String())
	//app.Debugf("run scp: %#v\n", scpCmd)
	err = scpCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("download file '%s': %v", remotePath, err)
	}

	data, err := ioutil.ReadFile(dstPath)
	if err != nil {
		return nil, fmt.Errorf("reading tmp file '%s': %v", dstPath, err)
	}

	return data, nil
}

func CreateEmptyTmpFile() (string, error) {
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("deckhouse-candi-scp-%d-%s.tmp", os.Getpid(), uuid.NewV4().String()))

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
