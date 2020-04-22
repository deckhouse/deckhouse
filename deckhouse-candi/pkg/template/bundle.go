package template

import (
	"fmt"
	"gopkg.in/yaml.v2"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
)

func PrepareBundle(templateController *TemplateController, nodeIP, bundleName string, metaConfig *config.MetaConfig) error {
	kubeadmData := metaConfig.MarshalConfigForKubeadmTemplates(nodeIP)
	kubeadmDataFormatted, _ := yaml.Marshal(kubeadmData)

	logboek.LogInfoF("Kubeadm data:\n---\n%s\n\n", kubeadmDataFormatted)
	if err := templateController.RenderAndSaveTemplates(
		"/deckhouse/candi/control-plane-kubeadm/",
		"/var/lib/bashible/kubeadm/",
		kubeadmData,
	); err != nil {
		return err
	}

	if err := templateController.RenderAndSaveTemplates(
		"/deckhouse/candi/control-plane-kubeadm/kustomize",
		"/var/lib/bashible/kubeadm/kustomize/",
		kubeadmData,
	); err != nil {
		return err
	}

	bashibleData := metaConfig.MarshalConfigForBashibleBundleTemplate(bundleName, nodeIP)
	bashibleDataFormatted, _ := yaml.Marshal(bashibleData)

	logboek.LogInfoF("Bashible data:\n---\n%s\n\n", bashibleDataFormatted)

	if err := templateController.RenderAndSaveTemplates(
		"/deckhouse/candi/bashible",
		"/var/lib/bashible/",
		bashibleData,
	); err != nil {
		return err
	}

	if err := templateController.RenderAndSaveTemplates(
		"/deckhouse/candi/bashible/common-steps/all/",
		"/var/lib/bashible/bundle_steps/",
		bashibleData,
	); err != nil {
		return err
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		if err := templateController.RenderAndSaveTemplates(
			fmt.Sprintf("/deckhouse/candi/bashible/bundles/%s/%s/", bundleName, steps),
			"/var/lib/bashible/bundle_steps/",
			bashibleData,
		); err != nil {
			return err
		}
	}

	for _, steps := range []string{"all", "cluster-bootstrap"} {
		if err := templateController.RenderAndSaveTemplates(
			fmt.Sprintf("/deckhouse/candi/cloud-providers/%s/bashible-bundles/%s/%s/", metaConfig.ProviderName, bundleName, steps),
			"/var/lib/bashible/bundle_steps/",
			bashibleData,
		); err != nil {
			return err
		}
	}

	if err := templateController.RenderBashBooster(
		"/deckhouse/candi/bashible/bashbooster",
		"/var/lib/bashible/",
	); err != nil {
		return err
	}

	return nil
}
