// Example of using d8sql as a library.
//
// It builds an Engine from the standard kubeconfig and runs a few queries,
// printing the results. The same Engine instance is reused across queries so
// that the discovery/REST-mapper cache is shared.
//
// Run with:
//
//	go run ./example
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/deckhouse/d8sql"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Build a *rest.Config from the default kubeconfig discovery rules.
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	cfg, err := cc.ClientConfig()
	if err != nil {
		log.Fatalf("load kubeconfig: %v", err)
	}

	// Construct the engine once. Resource resolution is cached on the instance.
	// By default queries span all namespaces; pass
	// d8sql.WithDefaultNamespace("ns") to scope them.
	// (Alternatively, use d8sql.New(dynClient, restMapper) to inject your own
	// already-configured, cached clients.)
	engine, err := d8sql.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("create engine: %v", err)
	}

	ctx := context.Background()

	queries := []string{
		// Running pods across all namespaces.
		"SELECT metadata.name, metadata.namespace, status.phase FROM pods WHERE status.phase = 'Running'",
		// Services whose name contains "redis".
		"SELECT metadata.name FROM services WHERE metadata.name LIKE '%redis%'",
		// Cluster-scoped resource.
		"SELECT metadata.name FROM nodes",
		// JOIN + label filter: all pods on nodes in the "worker" node group.
		"SELECT pod.metadata.namespace, pod.metadata.name, pod.spec.nodeName " +
			"FROM pod JOIN node ON pod.spec.nodeName == node.metadata.name " +
			"WHERE node.metadata.labels.'node.deckhouse.io/group' = 'worker'",
	}

	for _, q := range queries {
		fmt.Printf("\n--- %s\n", q)
		res, err := engine.ExecuteOne(ctx, q)
		if err != nil {
			log.Printf("query failed: %v", err)
			continue
		}
		printResult(res)
	}
}

func printResult(res d8sql.Result) {
	if res.Columns != nil {
		fmt.Println(res.Columns)
		for _, r := range res.Rows {
			fmt.Println(r)
		}
		return
	}
	for _, o := range res.Objects {
		fmt.Printf("%s/%s\n", o.GetNamespace(), o.GetName())
	}
}
