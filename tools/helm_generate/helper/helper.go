package helper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const defaultPerm = 0777

// DeckhouseRoot get deckhouse root dirrectory.
func DeckhouseRoot() (path string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	r := regexp.MustCompile("/deckhouse")
	sub := r.Split(cwd, 2)
	if len(sub) != 2 {
		return "", errors.New("dir: Incorrect utility launch directory")
	}
	root := filepath.Join(sub[0], "/deckhouse")

	return root, err
}

// NewRenderDir create a new temporary directory following the defalt helm template.
func NewRenderDir(chartName string) (path string, err error) {
	renderdir, err := os.MkdirTemp("", "renderdir")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(renderdir, "/charts"), defaultPerm); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(renderdir, "/templates"), defaultPerm); err != nil {
		return "", err
	}

	chartData := fmt.Sprintf("name: %s\nversion: 0.0.1", chartName)
	if err := os.WriteFile(filepath.Join(renderdir, "Chart.yaml"), []byte(chartData), defaultPerm); err != nil {
		return "", err
	}

	deckhouseRoot, err := DeckhouseRoot()
	if err != nil {
		return "", err
	}
	helmLibPath := "helm_lib/charts/deckhouse_lib_helm"

	if err := os.Symlink(filepath.Join(deckhouseRoot, helmLibPath), filepath.Join(renderdir, "/charts/helm_lib")); err != nil {
		return "", err
	}

	return renderdir, nil
}
