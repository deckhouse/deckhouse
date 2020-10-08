package commands

import (
	"fmt"
	"os"
	"time"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
)

func DefineTestKubernetesAPIConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("kubernetes-api-connection", "Test connection to kubernetes api via ssh or directly.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		// app.AskBecomePass = true
		err = operations.AskBecomePassword()
		if err != nil {
			return err
		}

		kubeCl := client.NewKubernetesClient().WithSSHClient(sshCl)
		// auto init
		err = kubeCl.Init("")
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}

		list, err := kubeCl.CoreV1().Namespaces().List(v1.ListOptions{})
		if err != nil {
			log.InfoF("list namespaces: %v", err)
			if kubeCl.KubeProxy != nil {
				log.InfoLn("Press Ctrl+C to close proxy connection.")
				ch := make(chan struct{})
				<-ch
			}
			return nil
		}

		if len(list.Items) > 0 {
			log.InfoLn("Namespaces:")
			for _, ns := range list.Items {
				log.InfoF("  ns/%s\n", ns.Name)
			}
		} else {
			log.InfoLn("No namespaces.")
		}

		TestCommandDelay()

		return nil
	})
	return cmd
}

func DefineWaitDeploymentReadyCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("deployment-ready", "Wait while deployment is ready.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

	var Namespace string
	var Name string

	cmd.Flag("namespace", "Use namespace").
		StringVar(&Namespace)
	cmd.Flag("name", "Deployment name").
		StringVar(&Name)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = operations.AskBecomePassword()
		if err != nil {
			return err
		}

		err = log.Process("bootstrap", "Wait for Deckhouse to become Ready", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshCl)
			// auto init
			err = kubeCl.Init("")
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

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

func TestCommandDelay() {
	delayStr := os.Getenv("TEST_DELAY")
	if delayStr == "" || delayStr == "no" {
		return
	}

	delay, err := time.ParseDuration(delayStr)

	if err != nil {
		delay = time.Minute
	}

	time.Sleep(delay)
}
