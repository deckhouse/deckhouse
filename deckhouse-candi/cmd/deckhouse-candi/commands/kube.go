package commands

import (
	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"fmt"
	"github.com/flant/logboek"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DefineTestKubernetesAPIConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("kubernetes-api-connection", "Test connection to kubernetes api via ssh or directly.")
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

func DefineWaitDeploymentReadyCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("deckhouse-deployment-ready", "Wait while deployment is ready.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

	var Namespace string
	var Name string

	cmd.Flag("namespace", "Use namespace").
		StringVar(&Namespace)
	cmd.Flag("name", "Deployment name").
		StringVar(&Name)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().StartSession()
		defer sshCl.StopSession()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		err = logboek.LogProcess("🛥️ Wait for Deckhouse to become ready 🛥️", log.TaskOptions(), func() error {
			kubeCl := kube.NewKubernetesClient().WithSshClient(sshCl)
			// auto init
			err = kubeCl.Init("")
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			// defer stop ssh-agent, proxy and a tunnel
			defer kubeCl.Stop()

			err = deckhouse.WaitForReadiness(kubeCl, &deckhouse.Config{})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	return cmd
}
