package commands

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/deckhouse-cluster/pkg/app"
	"flant/deckhouse-cluster/pkg/kube"
	"flant/deckhouse-cluster/pkg/ssh"
)

func GetTestSshConnectionCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	return cmd.Command("test-ssh-connection", "Test connection via ssh.").
		Action(func(c *kingpin.ParseContext) error {
			privateKeys, err := ssh.ParseSshPrivateKeyPaths(app.SshAgentPrivateKeys)
			if err != nil {
				return fmt.Errorf("ssh private keys: %v", err)
			}
			sshCl := ssh.SshClient{
				BastionHost: app.SshBastionHost,
				BastionUser: app.SshBastionUser,
				PrivateKeys: privateKeys,
				ExtraArgs:   app.SshExtraArgs,
			}

			app.Debugf("ssh client config: %+v\n", sshCl)

			err = sshCl.StartSshAgent()
			if err != nil {
				return fmt.Errorf("start ssh-agent: %v", err)
			}
			defer sshCl.StopSshAgent()

			err = sshCl.AddKeys()
			if err != nil {
				return fmt.Errorf("add keys: %v", err)
			}

			sshCl.Host = app.SshHost
			sshCl.User = app.SshUser

			out, err := sshCl.Command("ls", "-la").CombinedOutput()
			fmt.Printf("ls -la: %s\n", out)
			//lsCmd := exec.Command("ls", "-la")
			//err = sshCl.ExecuteCmd(lsCmd)
			if err != nil {
				return fmt.Errorf("ls -la: %v", err)
			}

			return nil
		})
}

func GetTestKubernetesAPIConnectionCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	return cmd.Command("test-kubernetes-api-connection", "Test connection to kubernetes api via ssh or directly.").
		Action(func(c *kingpin.ParseContext) error {
			kubeCl := kube.NewKubernetesClient()
			// auto init
			err := kubeCl.Init("")
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
}
