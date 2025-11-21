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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	ssh_testing "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh/testing"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/stretchr/testify/require"
)

func TestSSHFileUpload(t *testing.T) {
	testName := "TestSSHFileUpload"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20020, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	// creating direactory to upload
	testDir := filepath.Join(os.TempDir(), "dhctltests", "upload")
	err = os.MkdirAll(testDir, 0755)
	if err != nil {
		// cannot start test w/o files to upload
		return
	}

	testFile, err := os.CreateTemp(testDir, "upload")
	if err != nil {
		// cannot start test w/o files to upload
		return
	}
	testFile.WriteString("Hello world")
	// create some files for recursive upload
	os.CreateTemp(testDir, "second")
	os.CreateTemp(testDir, "third")

	symlink := filepath.Join(os.TempDir(), "new-test-file")
	err = os.Symlink(testFile.Name(), symlink)
	if err != nil {
		// cannot start test w/o symlink
		os.Remove(symlink)
		os.Symlink(testFile.Name(), symlink)
		// return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20020"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(context.Background(), settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		os.RemoveAll(testDir)
		os.Remove(symlink)
	})
	t.Run("Upload files and directories to container via existing ssh client", func(t *testing.T) {
		cases := []struct {
			title   string
			srcPath string
			dstPath string
			wantErr bool
			err     string
		}{
			{
				title:   "Single file",
				srcPath: testFile.Name(),
				dstPath: ".",
				wantErr: false,
			},
			{
				title:   "Directory",
				srcPath: testDir,
				dstPath: "/tmp",
				wantErr: false,
			},
			{
				title:   "Nonexistent",
				srcPath: "/path/to/nonexistent/flie",
				dstPath: "/tmp",
				wantErr: true,
				err:     "failed to open local file",
			},
			{
				title:   "File to root",
				srcPath: testFile.Name(),
				dstPath: "/any",
				wantErr: true,
			},
			{
				title:   "File to /var/lib",
				srcPath: testFile.Name(),
				dstPath: "/var/lib",
				wantErr: true,
			},
			{
				title:   "File to unaccessible file",
				srcPath: testFile.Name(),
				dstPath: "/path/what/not/exists.txt",
				wantErr: true,
				err:     "failed to copy file to remote host",
			},
			{
				title:   "Directory to root",
				srcPath: testDir,
				dstPath: "/",
				wantErr: true,
			},
			{
				title:   "Symlink",
				srcPath: symlink,
				dstPath: ".",
				wantErr: false,
			},
			{
				title:   "Device",
				srcPath: "/dev/zero",
				dstPath: "/",
				wantErr: true,
				err:     "is not a directory or file",
			},
			{
				title:   "Unaccessible dir",
				srcPath: "/var/audit",
				dstPath: ".",
				wantErr: true,
				err:     "could not read directory",
			},
			{
				title:   "Unaccessible file",
				srcPath: "/etc/sudoers",
				dstPath: ".",
				wantErr: true,
				err:     "failed to open local file",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				f := sshClient.File()
				err = f.Upload(context.Background(), c.srcPath, c.dstPath)
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})

	t.Run("Equality of uploaded and local file content", func(t *testing.T) {
		f := sshClient.File()
		err := f.Upload(context.Background(), testFile.Name(), "/tmp/testfile.txt")
		// testFile contains "Hello world" string
		require.NoError(t, err)

		sess, err := sshClient.GetClient().NewSession()
		require.NoError(t, err)
		defer sess.Close()
		out, err := sess.Output("cat /tmp/testfile.txt")
		require.NoError(t, err)
		// out contains a contant of uploaded file, should be equal to testFile contant
		require.Equal(t, "Hello world", string(out))

	})
	t.Run("Equality of uploaded and local directory", func(t *testing.T) {
		f := sshClient.File()
		err := f.Upload(context.Background(), testDir, "/tmp/upload")
		require.NoError(t, err)

		cmd := exec.Command("ls", testDir)
		lsResult, err := cmd.Output()
		require.NoError(t, err)

		sess, err := sshClient.GetClient().NewSession()
		require.NoError(t, err)
		defer sess.Close()
		out, err := sess.Output("ls /tmp/upload")
		require.NoError(t, err)
		// out contains a result of ls command execution, should be equal to local ls execution
		require.Equal(t, string(lsResult), string(out))

	})
}

func TestSSHFileUploadBytes(t *testing.T) {
	testName := "TestSSHFileUploadBytes"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20020, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}
	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20020"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(context.Background(), settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
	})
	app.TmpDirName = os.TempDir()

	t.Run("Upload bytes", func(t *testing.T) {
		f := sshClient.File()
		err := f.UploadBytes(context.Background(), []byte("Hello world"), "/tmp/testfile.txt")
		require.NoError(t, err)

		sess, err := sshClient.GetClient().NewSession()
		require.NoError(t, err)
		defer sess.Close()
		out, err := sess.Output("cat /tmp/testfile.txt")
		require.NoError(t, err)
		// out contains a contant of uploaded file, should be equal to testFile contant
		require.Equal(t, "Hello world", string(out))
	})

}

func TestCreateEmptyTmpFile(t *testing.T) {
	t.Run("Creating empty temp file", func(t *testing.T) {
		cases := []struct {
			title      string
			tmpDirName string
			wantErr    bool
			err        string
		}{
			{
				title:      "Accessible tmp",
				tmpDirName: os.TempDir(),
				wantErr:    false,
			},
			{
				title:      "Unaccessible tmp",
				tmpDirName: "/var/lib",
				wantErr:    true,
				err:        "permission denied",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				app.TmpDirName = c.tmpDirName
				uid := os.Geteuid()
				if uid == 0 && c.wantErr {
					t.Skip("Test TestCreateEmptyTmpFile was skipped, cannot try to access unaccessible dir from root user")
				}
				filename, err := CreateEmptyTmpFile()
				if !c.wantErr {
					require.NoError(t, err)
					os.Remove(filename)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})
}

func TestSSHFileDownload(t *testing.T) {
	testName := "TestSSHFileDownload"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20020, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}

	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20020"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(context.Background(), settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	// preparing some test related data
	err = sshClient.Command("mkdir  -p /tmp/testdata").Run(context.Background())
	require.NoError(t, err)
	err = sshClient.Command("echo \"Some test data\" > /tmp/testdata/first").Run(context.Background())
	require.NoError(t, err)
	err = sshClient.Command("touch /tmp/testdata/second").Run(context.Background())
	require.NoError(t, err)
	err = sshClient.Command("touch /tmp/testdata/third").Run(context.Background())
	require.NoError(t, err)
	err = sshClient.Command("ln -s /tmp/testdata/first /tmp/link").Run(context.Background())
	require.NoError(t, err)

	// creating direactory to upload
	testDir := filepath.Join(os.TempDir(), "dhctltests", "download")
	err = os.MkdirAll(testDir, 0755)
	if err != nil {
		// cannot start test w/o files to download
		return
	}

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
		os.RemoveAll(testDir)
	})
	t.Run("Download files and directories to container via existing ssh client", func(t *testing.T) {
		cases := []struct {
			title   string
			srcPath string
			dstPath string
			wantErr bool
			err     string
		}{
			{
				title:   "Single file",
				srcPath: "/tmp/testdata/first",
				dstPath: testDir,
				wantErr: false,
			},
			{
				title:   "Directory",
				srcPath: "/tmp/testdata",
				dstPath: testDir + "/downloaded",
				wantErr: false,
			},
			{
				title:   "Nonexistent",
				srcPath: "/path/to/nonexistent/flie",
				dstPath: "/tmp",
				wantErr: true,
			},
			{
				title:   "File to root",
				srcPath: "/tmp/testdata/first",
				dstPath: "/any",
				wantErr: true,
			},
			{
				title:   "File to /var/lib",
				srcPath: "/tmp/testdata/first",
				dstPath: "/var/lib",
				wantErr: true,
			},
			{
				title:   "File to unaccessible file",
				srcPath: "/tmp/testdata/first",
				dstPath: "/path/what/not/exists.txt",
				wantErr: true,
				err:     "failed to open local file",
			},
			{
				title:   "Directory to root",
				srcPath: "/tmp/testdata",
				dstPath: "/",
				wantErr: true,
			},
			{
				title:   "Symlink",
				srcPath: "/tmp/link",
				dstPath: testDir,
				wantErr: false,
			},
			{
				title:   "Device",
				srcPath: "/dev/zero",
				dstPath: "/",
				wantErr: true,
				err:     "failed to open local file",
			},
			{
				title:   "Unaccessible dir",
				srcPath: "/var/audit",
				dstPath: testDir,
				wantErr: true,
			},
			{
				title:   "Unaccessible file",
				srcPath: "/etc/sudoers",
				dstPath: testDir,
				wantErr: true,
				err:     "failed to copy file from remote host",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				f := sshClient.File()
				err = f.Download(context.Background(), c.srcPath, c.dstPath)
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
			})
		}
	})

	t.Run("Equality of downloaded and remote file content", func(t *testing.T) {
		f := sshClient.File()
		err := f.Download(context.Background(), "/tmp/testdata/first", "/tmp/testfile.txt")
		// /tmp/testdata/first contains "Some test data" string
		require.NoError(t, err)

		cmd := exec.Command("cat", "/tmp/testfile.txt")
		out, err := cmd.Output()
		require.NoError(t, err)
		// out contains a contant of uploaded file, should be equal to testFile contant
		require.Equal(t, "Some test data\n", string(out))
		os.Remove("/tmp/testfile.txt")
	})
	t.Run("Equality of downloaded and remote direcroty", func(t *testing.T) {
		f := sshClient.File()
		err = f.Download(context.Background(), "/tmp/testdata", "/tmp")
		require.NoError(t, err)

		cmd := exec.Command("ls", "/tmp/testdata")
		lsResult, err := cmd.Output()
		require.NoError(t, err)

		sess, err := sshClient.GetClient().NewSession()
		require.NoError(t, err)
		defer sess.Close()
		out, err := sess.Output("ls /tmp/testdata")
		require.NoError(t, err)
		// out contains a result of ls command execution, should be equal to local ls execution
		require.Equal(t, string(lsResult), string(out))
		os.RemoveAll("/tmp/testdata")

	})
}

func TestSSHFileDownloadBytes(t *testing.T) {
	testName := "TestSSHFileDownloadBytes"

	if os.Getenv("SKIP_GOSSH_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}
	// genetaring ssh keys
	path, publicKey, err := ssh_testing.GenerateKeys("")
	if err != nil {
		return
	}

	// starting openssh container with password auth
	container := ssh_testing.NewSSHContainer(publicKey, "", "", "user", 20020, true)
	err = container.Start()
	if err != nil {
		// cannot start test w/o container
		return
	}
	os.Setenv("SSH_AUTH_SOCK", "")
	settings := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "localhost", Name: "localhost"}},
		User:           "user",
		Port:           "20020"})
	keys := []session.AgentPrivateKey{{Key: path}}
	sshClient := NewClient(context.Background(), settings, keys)
	err = sshClient.Start()
	// expecting no error on client start
	require.NoError(t, err)

	// preparing file to download
	err = sshClient.Command("echo \"Some test data\" > /tmp/testfile").Run(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		sshClient.Stop()
		container.Stop()
		os.Remove(path)
	})

	t.Run("Download bytes", func(t *testing.T) {
		cases := []struct {
			title      string
			remotePath string
			tmpDirName string
			wantErr    bool
			err        string
		}{
			{
				title:      "Positive result",
				remotePath: "/tmp/testfile",
				tmpDirName: os.TempDir(),
				wantErr:    false,
			},
			{
				title:      "Unaccessible tmp",
				remotePath: "/tmp/testfile",
				tmpDirName: "/var/lib",
				wantErr:    true,
				err:        "create target tmp file",
			},
			{
				title:      "Unaccessible remote file",
				remotePath: "/etc/sudoers",
				tmpDirName: os.TempDir(),
				wantErr:    true,
				err:        "download target tmp file",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				app.TmpDirName = c.tmpDirName
				f := sshClient.File()
				bytes, err := f.DownloadBytes(context.Background(), c.remotePath)
				if !c.wantErr {
					require.NoError(t, err)
					// out contains a contant of uploaded file, should be equal to testFile contant
					require.Equal(t, "Some test data\n", string(bytes))
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}
			})
		}

	})

}
