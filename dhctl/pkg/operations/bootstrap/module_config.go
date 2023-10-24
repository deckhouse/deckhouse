package bootstrap

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func createModuleConfig(kubeDynamicApi dynamic.Interface, mc *unstructured.Unstructured) error {
	moduleConfigName := mc.GetName()
	loop := retry.NewLoop(fmt.Sprintf("Create %q ModuleConfig", moduleConfigName), 15, time.Second*10)
	err := loop.Run(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := kubeDynamicApi.Resource(schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1alpha1",
			Resource: "moduleconfigs",
		}).Create(ctx, mc, v1.CreateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("cannot create %q ModuleConfig: %w", moduleConfigName, err)
	}
	return nil
}

// unlockBootstrapProcess deletes deckhouse-bootstrap-lock ConfigMap that prevents module hooks from executing from d8-system namespace.
func unlockBootstrapProcess(kubeCl *client.KubernetesClient) error {
	loop := retry.NewSilentLoop("Unlock bootstrap process after ModuleConfig's creation", 25, time.Second*5)
	err := loop.Run(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return kubeCl.CoreV1().
			ConfigMaps("d8-system").
			Delete(ctx, "deckhouse-bootstrap-lock", v1.DeleteOptions{})
	})
	if err != nil {
		return fmt.Errorf("cannot delete deckhouse-bootstrap-lock ConfigMap: %w", err)
	}

	return nil
}
