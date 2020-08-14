package commands

import (
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/system/ssh"
)

func DefineTestSshConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("ssh-connection", "Test connection via ssh.")
	app.DefineSshFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = sshCl.Check().AwaitAvailability()

		if err != nil {
			return fmt.Errorf("check connection: %v", err)
		}

		TestCommandDelay()

		return nil
	})
	return cmd
}

func DefineTestScpCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	var SrcPath string
	var DstPath string
	var Data string
	var Direction string
	cmd := parent.Command("scp", "Test scp file operations.")
	app.DefineSshFlags(cmd)
	cmd.Flag("src", "source path").Short('s').StringVar(&SrcPath)
	cmd.Flag("dst", "destination path").Short('d').StringVar(&DstPath)
	cmd.Flag("data", "data to test uploadbytes method").StringVar(&Data)
	cmd.Flag("way", "transfer direction: 'up' to upload to remote or 'down' to download from remote").Short('w').StringVar(&Direction)
	cmd.Action(func(c *kingpin.ParseContext) error {
		app.Debugf("scp: start ssh-agent\n")
		sshCl, err := ssh.NewClientFromFlags().Start()

		if err != nil {
			return err
		}

		app.Debugf("scp: start\n")

		success := false
		if Direction == "up" {
			if Data != "" {
				fmt.Printf("upload bytes to '%s' on remote\n", DstPath)
				err = sshCl.File().UploadBytes([]byte(Data), DstPath)
			} else {
				fmt.Printf("upload local '%s' to '%s' on remote\n", SrcPath, DstPath)
				err = sshCl.File().Upload(SrcPath, DstPath)
			}
			if err != nil {
				return err
			}
			success = true
		} else {
			if DstPath == "stdout" {
				fmt.Printf("download bytes from remote '%s'\n", SrcPath)
				data, err := sshCl.File().DownloadBytes(SrcPath)
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				success = true
			} else {
				fmt.Printf("download bytes from remote '%s' to local '%s'\n", SrcPath, DstPath)
				err = sshCl.File().Download(SrcPath, DstPath)
				if err != nil {
					return err
				}
				success = true
			}
		}

		if !success {
			fmt.Printf("unrecognized flags\n")
		}

		return nil
	})

	return cmd
}

func DefineTestUploadExecCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	var ScriptPath string
	var Sudo bool
	cmd := parent.Command("upload-exec", "Test scp upload and ssh run uploaded script.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	cmd.Flag("script", "source path").
		StringVar(&ScriptPath)
	cmd.Flag("sudo", "source path").Short('s').
		BoolVar(&Sudo)

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := app.AskBecomePassword()
		if err != nil {
			return err
		}

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return nil
		}

		cmd := sshClient.UploadScript(ScriptPath)
		if Sudo {
			cmd = cmd.Sudo()
		}
		var stdout []byte
		stdout, err = cmd.Execute()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("script '%s' error: %w stderr: %s", ScriptPath, err, string(ee.Stderr))
			} else {
				return fmt.Errorf("script '%s' error: %w", ScriptPath, err)
			}
		}
		logboek.LogInfoF("stdout: %s\n", strings.Trim(string(stdout), "\n "))
		logboek.LogInfoF("Got %d symbols\n", len(stdout))
		return nil
	})

	return cmd
}

func DefineTestBundle(parent *kingpin.CmdClause) *kingpin.CmdClause {
	var ScriptName string
	var BundleDir string

	cmd := parent.Command("bashible-bundle", "Test upload and execute a bundle.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	cmd.Flag("bundle-dir", "path of a bundle root directory").
		Short('d').
		StringVar(&BundleDir)
	cmd.Flag("bundle-script", "path of a bundle main script").
		Short('s').
		StringVar(&ScriptName)

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := app.AskBecomePassword()
		if err != nil {
			return err
		}

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return nil
		}

		cmd := sshClient.UploadScript(ScriptName).Sudo()
		parentDir := path.Dir(BundleDir)
		bundleDir := path.Base(BundleDir)
		stdout, err := cmd.ExecuteBundle(parentDir, bundleDir)
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("bundle '%s' error: %w\nstderr: %s\n", bundleDir, err, string(ee.Stderr))
			} else {
				return fmt.Errorf("bundle '%s' error: %w", bundleDir, err)
			}
		}
		logboek.LogInfoF("Got %d symbols\n", len(stdout))
		return nil
	})

	return cmd
}
