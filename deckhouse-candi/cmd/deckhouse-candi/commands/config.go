package commands

import (
	"os"
	"strings"

	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/template"
)

const (
	bashibleTemplateOpenAPI = "/deckhouse/candi/bashible/openapi.yaml"
	kubeadmTemplateOpenAPI  = "/deckhouse/candi/control-plane-kubeadm/openapi.yaml"
)

func DefineRenderBashibleBundle(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("bashible-bundle", "Render bashible bundle.")
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func() error {
		templateData, err := config.ParseBashibleConfig(app.ConfigPath, bashibleTemplateOpenAPI)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		logboek.LogInfoF("Bundle Dir: %q\n\n", templateController.TmpDir)

		return template.PrepareBashibleBundle(
			templateController,
			templateData,
			strings.ToLower(templateData["provider"].(string)),
			templateData["bundle"].(string),
		)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := logboek.LogProcess("📦 Prepare Bashible Bundle 📦",
			log.MainProcessOptions(), func() error { return runFunc() })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}

func DefineRenderKubeadmConfig(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("kubeadm-config", "Render kubeadm config.")
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func() error {
		templateData, err := config.ParseBashibleConfig(app.ConfigPath, kubeadmTemplateOpenAPI)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		logboek.LogInfoF("Bundle Dir: %q\n\n", templateController.TmpDir)

		return template.PrepareKubeadmConfig(templateController, templateData)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := logboek.LogProcess("📦 Prepare Kubeadm Config 📦",
			log.MainProcessOptions(), func() error { return runFunc() })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}
