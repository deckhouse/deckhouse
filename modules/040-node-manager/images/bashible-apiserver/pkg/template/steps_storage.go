package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// NameMapper maps the name of resource to the secretKey of a template
type NameMapper func(name string) (string, error)

type config struct {
	rootDir  string
	provider string
	target   string
}

// StepsStorage is the storage if bashible steps for a particular steps target
type StepsStorage struct {
	config config
}

// NewStepsStorage creates StepsStorage for target and cloud provider.
//
// provider = aws | gcp | openstack | ...
// target   = all | node-group
func NewStepsStorage(rootDir, provider, target string) *StepsStorage {
	config := config{rootDir, provider, target}
	return &StepsStorage{config}
}

func (s *StepsStorage) Render(templateContext map[string]interface{}) (map[string]string, error) {
	bundle, ok := templateContext["bundle"].(string)
	if !ok {
		panic("expected string in templateContext[\"bundle\"]")
	}

	templates, err := s.readBundleTemplates(bundle)
	if err != nil {
		return nil, err
	}

	steps := map[string]string{}
	for name, content := range templates {

		step, err := RenderTemplate(name, content, templateContext)
		if err != nil {
			return nil, fmt.Errorf("cannot render template \"%s\" for bundle \"%s\": %v", name, bundle, err)
		}
		steps[step.FileName] = step.Content.String()
	}

	return steps, nil
}

func (s *StepsStorage) readBundleTemplates(bundle string) (map[string][]byte, error) {
	templates := map[string][]byte{}

	for _, dir := range s.lookupDirs(bundle) {

		err := readTemplates(dir, templates)
		if err != nil {
			return nil, fmt.Errorf("cannot read template in dir %s: %v", dir, err)
		}
	}

	return templates, nil
}

// Expected fs hierarchy so far
//      bashible/{bundle}/{target}
//      bashible/common-steps/{target}
//      cloud-providers/{provider}/bashible/{bundle}/{target}
//      cloud-providers/{provider}/bashible/common-steps/{target}
//
// Where
//      bundle   = "centos-7" | "ubuntu-lts"
//      target   = "all" | "node-group"
//      provider = "" | "aws" | "gcp" | "openstack" | ...
func (s *StepsStorage) lookupDirs(bundle string) []string {
	root := s.config.rootDir
	target := s.config.target
	provider := s.config.provider

	dirs := []string{
		filepath.Join(root, "bashible", "bundles", bundle, target),
		filepath.Join(root, "bashible", "common-steps", target),
	}

	// Are we in the cloud?
	if provider != "" {
		dirs = append(dirs,
			filepath.Join(root, "cloud-providers", provider, "bashible", "bundles", bundle, target),
			filepath.Join(root, "cloud-providers", provider, "bashible", "common-steps", target),
		)
	}

	return dirs
}

func readTemplates(rootDir string, templates map[string][]byte) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}

		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".sh.tpl") {
			// not template
			return nil
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		templates[info.Name()] = content
		return nil
	})
}
