// Command d8sql runs SQL statements against the cluster pointed to by the
// standard kubeconfig.
//
// Usage:
//
//	d8sql [flags] "SQL STATEMENT; [SQL STATEMENT; ...]"
//
// Flags:
//
//	-n, --namespace string   namespace to scope queries (default: all namespaces)
//	-o, --output string      output format: table | yaml | json (default "table")
//	    --kubeconfig string   path to kubeconfig (defaults to $KUBECONFIG or ~/.kube/config)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/d8sql"
	"github.com/deckhouse/d8sql/sql"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		namespace  = "" // empty => all namespaces
		output     = "table"
		kubeconfig = ""
		query      string
	)

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-n" || a == "--namespace":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			namespace = args[i]
		case strings.HasPrefix(a, "--namespace="):
			namespace = strings.TrimPrefix(a, "--namespace=")
		case a == "-o" || a == "--output":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			output = args[i]
		case strings.HasPrefix(a, "--output="):
			output = strings.TrimPrefix(a, "--output=")
		case a == "--kubeconfig":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			kubeconfig = args[i]
		case strings.HasPrefix(a, "--kubeconfig="):
			kubeconfig = strings.TrimPrefix(a, "--kubeconfig=")
		case a == "-h" || a == "--help":
			usage()
			return nil
		default:
			if query != "" {
				return fmt.Errorf("unexpected argument %q (only one query is allowed)", a)
			}
			query = a
		}
		i++
	}

	if query == "" {
		usage()
		return fmt.Errorf("no SQL query provided")
	}

	cfg, err := loadConfig(kubeconfig)
	if err != nil {
		return err
	}
	engine, err := d8sql.NewForConfig(cfg, d8sql.WithDefaultNamespace(namespace))
	if err != nil {
		return err
	}

	results, err := engine.Execute(context.Background(), query)
	if err != nil {
		return err
	}

	for idx, res := range results {
		if idx > 0 {
			fmt.Println()
		}
		if err := printResult(os.Stdout, res, output); err != nil {
			return err
		}
	}
	return nil
}

func loadConfig(explicit string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if explicit != "" {
		rules.ExplicitPath = explicit
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return cfg, nil
}

func printResult(w *os.File, res d8sql.Result, format string) error {
	switch res.Kind {
	case sql.StmtUpdate:
		fmt.Fprintf(w, "UPDATE %d\n", res.Affected)
		return nil
	case sql.StmtDelete:
		fmt.Fprintf(w, "DELETE %d\n", res.Affected)
		return nil
	case sql.StmtAssert:
		fmt.Fprintf(w, "ASSERT OK (%d matched)\n", res.Affected)
		return nil
	case sql.StmtIf:
		// Print what the taken branch produced, statement by statement.
		for i, nested := range res.Nested {
			if i > 0 {
				fmt.Fprintln(w)
			}
			if err := printResult(w, nested, format); err != nil {
				return err
			}
		}
		return nil
	}

	// SELECT
	switch format {
	case "yaml":
		return printObjectsYAML(w, res.Objects)
	case "json":
		return printObjectsJSON(w, res.Objects)
	}

	if res.Columns == nil {
		// SELECT * -> default NAMESPACE/NAME table or objects fallback
		return printDefaultTable(w, res.Objects)
	}
	return printTable(w, res.Columns, res.Rows)
}

func printTable(w *os.File, cols []string, rows [][]any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, strings.ToUpper(c))
	}
	fmt.Fprintln(tw)
	for _, r := range rows {
		for i, cell := range r {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, formatCell(cell))
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

func printDefaultTable(w *os.File, objs []*unstructured.Unstructured) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAMESPACE\tNAME")
	for _, o := range objs {
		fmt.Fprintf(tw, "%s\t%s\n", o.GetNamespace(), o.GetName())
	}
	return tw.Flush()
}

func printObjectsYAML(w *os.File, objs []*unstructured.Unstructured) error {
	for i, o := range objs {
		if i > 0 {
			fmt.Fprintln(w, "---")
		}
		b, err := sigsyaml.Marshal(o.Object)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func printObjectsJSON(w *os.File, objs []*unstructured.Unstructured) error {
	list := make([]map[string]any, len(objs))
	for i, o := range objs {
		list[i] = o.Object
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(list)
}

func formatCell(v any) string {
	switch x := v.(type) {
	case nil:
		return "<none>"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(b)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `d8sql - run SQL queries against Kubernetes resources

Usage:
  d8sql [flags] "SQL STATEMENT"

Flags:
  -n, --namespace string    namespace to scope queries (default: all namespaces)
  -o, --output string       output format: table | yaml | json (default "table")
      --kubeconfig string   path to kubeconfig
  -h, --help                show this help

Examples:
  d8sql "SELECT metadata.name, status.phase FROM pods WHERE status.phase = 'Running'"
  d8sql -n kube-system "SELECT * FROM pods"
  d8sql "UPDATE deployment SET spec.replicas = 1 WHERE metadata.namespace = 'default' AND spec.replicas < 1"
  d8sql "ASSERT EMPTY (SELECT metadata.name FROM pods WHERE status.phase = 'Failed') FAIL 'FAILED_PODS'"`)
}
