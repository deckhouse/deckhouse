// Example of using d8sql as a library.
//
// It builds an Engine from the standard kubeconfig and runs a few queries,
// printing the results. The same Engine instance is reused across queries so
// that the discovery/REST-mapper cache is shared.
//
// Run with:
//
//	go run ./example           # read-only queries
//	go run ./example -write    # also runs the INSERT/UPDATE/DELETE example
//
// The write half is opt-in on purpose: everything else here only reads, so
// running the example against a real cluster cannot change it by accident.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/d8sql"
	"github.com/deckhouse/d8sql/sql"
)

func main() {
	write := flag.Bool("write", false, "also run the INSERT/UPDATE/DELETE example (creates and removes a ConfigMap)")
	flag.Parse()

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

	if *write {
		writeExample(ctx, engine)
	}
}

// writeExample creates an object, changes it and removes it again, as one
// batch. The whole batch is parsed and compiled before the first statement
// runs, so a typo in the DELETE cannot leave the INSERT applied.
//
// The INSERT is wrapped in IF NOT EXISTS because INSERT surfaces the API
// server's AlreadyExists error: that guard is how a statement is made
// re-runnable, which is what a migration needs.
func writeExample(ctx context.Context, engine *d8sql.Engine) {
	const batch = `
IF NOT EXISTS (
  SELECT metadata.name FROM configmaps
  WHERE metadata.namespace = 'default' AND metadata.name = 'd8sql-example'
) THEN
  INSERT INTO configmaps SET
    metadata.name = 'd8sql-example',
    metadata.namespace = 'default',
    metadata.labels.'app.kubernetes.io/managed-by' = 'd8sql-example',
    data.hello = 'world';
END IF;

UPDATE configmaps SET data.hello = 'updated'
  WHERE metadata.namespace = 'default' AND metadata.name = 'd8sql-example';

DELETE FROM configmaps
  WHERE metadata.namespace = 'default' AND metadata.name = 'd8sql-example';
`

	fmt.Printf("\n--- write example (INSERT, UPDATE, DELETE)\n")

	results, err := engine.Execute(ctx, batch)
	if err != nil {
		log.Printf("write example failed: %v", err)

		return
	}

	for _, res := range results {
		printResult(res)
	}
}

func printResult(res d8sql.Result) {
	// An IF reports what its taken branch executed.
	if res.Kind == sql.StmtIf {
		if len(res.Nested) == 0 {
			fmt.Println("IF: no branch matched")
		}
		for _, nested := range res.Nested {
			printResult(nested)
		}

		return
	}

	switch res.Kind {
	case sql.StmtInsert, sql.StmtUpdate, sql.StmtDelete:
		fmt.Printf("%s: %d object(s)\n", statementName(res.Kind), res.Affected)
		for _, o := range res.Objects {
			fmt.Printf("  %s/%s\n", o.GetNamespace(), o.GetName())
		}

		return
	}

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

func statementName(kind sql.StmtKind) string {
	switch kind {
	case sql.StmtInsert:
		return "INSERT"
	case sql.StmtUpdate:
		return "UPDATE"
	case sql.StmtDelete:
		return "DELETE"
	default:
		return "statement"
	}
}
