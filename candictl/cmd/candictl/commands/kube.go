package commands

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/operations"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh"
)

func DefineTestKubernetesAPIConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("kubernetes-api-connection", "Test connection to kubernetes api via ssh or directly.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		var sshClient *ssh.Client
		var err error
		if app.SSHHost != "" {
			sshClient, err = ssh.NewClientFromFlags().Start()
			if err != nil {
				return err
			}

			err = operations.AskBecomePassword()
			if err != nil {
				return err
			}
		}

		kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
		// auto init
		err = kubeCl.Init()
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
	app.DefineKubeFlags(cmd)

	var Namespace string
	var Name string

	cmd.Flag("namespace", "Use namespace").
		StringVar(&Namespace)
	cmd.Flag("name", "Deployment name").
		StringVar(&Name)

	cmd.Action(func(c *kingpin.ParseContext) error {
		var sshClient *ssh.Client
		var err error
		if app.SSHHost != "" {
			sshClient, err = ssh.NewClientFromFlags().Start()
			if err != nil {
				return err
			}

			err = operations.AskBecomePassword()
			if err != nil {
				return err
			}
		}

		err = log.Process("bootstrap", "Wait for Deckhouse to become Ready", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			// auto init
			err = kubeCl.Init()
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.WaitForReadiness(kubeCl)
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
