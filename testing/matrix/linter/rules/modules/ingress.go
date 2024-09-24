package modules

import (
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

const (
	copyCustomCertificateImport = "github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"
)

func IngressHooksCheck(m utils.Module, object storage.StoreObject) errors.LintRuleError {
	if object.Unstructured.GetKind() != "Ingress" {
		return errors.EmptyRuleError
	}

	var imports = make(map[string]struct{})
	for _, hookPath := range collectGoHooks(m.Path) {
		p, err := getImports(hookPath)
		if err != nil {
			continue
		}
		maps.Copy(imports, p)
	}

	if _, ok := imports[copyCustomCertificateImport]; !ok {
		return errors.NewLintRuleError(
			"INGRESS",
			m.Name,
			nil,
			"Ingress does not contains copy_custom_certificate hook",
		)
	}

	return errors.EmptyRuleError
}

func collectGoHooks(moduleDir string) []string {
	goHooks := make([]string, 0)
	_ = filepath.Walk(moduleDir, func(path string, info os.FileInfo, err error) error {
		switch {
		case err != nil:
			return err

		case strings.HasSuffix(path, "test.go"): // ignore tests
			return nil

		case strings.HasSuffix(path, ".go"):
			goHooks = append(goHooks, path)

		default:
			return nil
		}

		return nil
	})

	return goHooks
}

func getImports(filename string) (map[string]struct{}, error) {
	fSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fSet, filename, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var imports = make(map[string]struct{})
	for _, s := range astFile.Imports {
		imports[trimQuotes(s.Path.Value)] = struct{}{}
	}

	return imports, nil
}

func trimQuotes(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == '"'
	})
}
