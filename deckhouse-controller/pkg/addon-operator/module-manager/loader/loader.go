package loader

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
)

const (
	ModuleDefinitionFile = "module.yaml"
)

// ModuleDefinition describes module, some extra data loaded from module.yaml
type ModuleDefinition struct {
	Name        string   `json:"name"`
	Tags        []string `json:"tags"`
	Weight      int      `json:"weight"`
	Description string   `json:"description"`
}

type DeckhouseModuleLoader struct {
	kubeClient *versioned.Clientset
}

func NewDeckhouseModuleLoader(config *rest.Config) (*DeckhouseModuleLoader, error) {
	mcClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &DeckhouseModuleLoader{kubeClient: mcClient}, nil
}

func (dml *DeckhouseModuleLoader) Pupupu() {
	releaseList, err := dml.kubeClient.DeckhouseV1alpha1().ModuleReleases().List(context.TODO(), v1.ListOptions{FieldSelector: "status.phase=Deployed"})
	if err != nil {
		fmt.Println("Err1", err)
	}

	fmt.Println("FOUND DEPLOyED releases", len(releaseList.Items))

	releaseList, err = dml.kubeClient.DeckhouseV1alpha1().ModuleReleases().List(context.TODO(), v1.ListOptions{LabelSelector: "status.phase=Deployed"})
	if err != nil {
		fmt.Println("ERR2", err)
	}

	fmt.Println("FOUND 2 DEPLOyED releases", len(releaseList.Items))

	// TODO: get all ModuleRelease with Deployed
	// TODO: check on file system
	// TODO: download if not compared
}
