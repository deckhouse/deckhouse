package commands

import (
	"fmt"
	"github.com/flant/logboek"
	"os/exec"
	"path"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sh_app "github.com/flant/shell-operator/pkg/app"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/ssh"
)

func DefineTestSshConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("test-ssh-connection", "Test connection via ssh.")
	app.DefineSshFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().StartSession()
		defer sshCl.StopSession()
		if err != nil {
			return err
		}

		err = sshCl.Check().AwaitAvailability()

		if err != nil {
			return fmt.Errorf("check connection: %v", err)
		}

		return nil
	})
	return cmd
}

func DefineTestKubernetesAPIConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("test-kubernetes-api-connection", "Test connection to kubernetes api via ssh or directly.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().StartSession()
		defer sshCl.StopSession()
		if err != nil {
			return err
		}

		//app.AskBecomePass = true
		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		kubeCl := kube.NewKubernetesClient().WithSshClient(sshCl)
		// auto init
		err = kubeCl.Init("")
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}
		// defer stop ssh-agent, proxy and a tunnel
		defer kubeCl.Stop()

		list, err := kubeCl.CoreV1().Namespaces().List(v1.ListOptions{})
		if err != nil {
			//return fmt.Errorf("list namespaces: %v", err)
			fmt.Printf("list namespaces: %v", err)
			if kubeCl.KubeProxy != nil {
				fmt.Printf("Press Ctrl+C to close proxy connection.")
				ch := make(chan struct{}, 0)
				<-ch
			}
			return nil
		}

		if len(list.Items) > 0 {
			fmt.Printf("Namespaces:\n")
			for _, ns := range list.Items {
				fmt.Printf("  ns/%s\n", ns.Name)
			}
		} else {
			fmt.Printf("No namespaces.\n")
		}

		return nil
	})
	return cmd
}

func DefineTestScpCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	var SrcPath string
	var DstPath string
	var Data string
	var Direction string
	cmd := parent.Command("test-scp", "Test scp file operations.")
	app.DefineSshFlags(cmd)
	cmd.Flag("src", "source path").Short('s').StringVar(&SrcPath)
	cmd.Flag("dst", "destination path").Short('d').StringVar(&DstPath)
	cmd.Flag("data", "data to test uploadbytes method").StringVar(&Data)
	cmd.Flag("way", "transfer direction: 'up' to upload to remote or 'down' to download from remote").Short('w').StringVar(&Direction)
	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().StartSession()
		defer sshCl.StopSession()
		if err != nil {
			return err
		}

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
	cmd := parent.Command("test-upload-exec", "Test scp upload and ssh run uploaded script.")
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

		sshClient, err := ssh.NewClientFromFlags().StartSession()
		if err != nil {
			return nil
		}
		defer sshClient.StopSession()

		cmd := sshClient.UploadScript(ScriptPath)
		if Sudo {
			cmd = cmd.Sudo()
		}
		var stdout []byte
		stdout, err = cmd.Execute()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("script '%s' error: %v\nstderr: %s\n", ScriptPath, err, string(ee.Stderr))
			} else {
				return fmt.Errorf("script '%s' error: %v\n", ScriptPath, err)
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

	cmd := parent.Command("test-bundle", "Test upload and execute a bundle.")
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

		sshClient, err := ssh.NewClientFromFlags().StartSession()
		if err != nil {
			return nil
		}
		defer sshClient.StopSession()

		cmd := sshClient.UploadScript(ScriptName).Sudo()
		parentDir := path.Dir(BundleDir)
		bundleDir := path.Base(BundleDir)
		stdout, err := cmd.ExecuteBundle(parentDir, bundleDir)
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("bundle '%s' error: %v\nstderr: %s\n", bundleDir, err, string(ee.Stderr))
			} else {
				return fmt.Errorf("bundle '%s' error: %v\n", bundleDir, err)
			}
		}
		logboek.LogInfoF("Got %d symbols\n", len(stdout))
		return nil
	})

	return cmd
}
