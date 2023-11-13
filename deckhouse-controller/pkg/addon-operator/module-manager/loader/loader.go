package loader

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func (dml *DeckhouseModuleLoader) Pupupu() error {
	externalModulesDir := os.Getenv("EXTERNAL_MODULES_DIR")
	if externalModulesDir == "" {
		log.Warn("EXTERNAL_MODULES_DIR is not set")
		return nil
	}
	// directory for symlinks will actual versions to all external-modules
	symlinksDir := filepath.Join(externalModulesDir, "modules")

	releaseList, err := dml.kubeClient.DeckhouseV1alpha1().ModuleReleases().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range releaseList.Items {
		if item.Status.Phase != "Deployed" {
			continue
		}

		moduleDir := filepath.Join(symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, item.Spec.ModuleName))
		_, err = os.Stat(moduleDir)
		if err != nil && os.IsNotExist(err) {
			moduleVersion := "v" + item.Spec.Version.String()
			moduleName := item.Spec.ModuleName
			moduleVersionPath := path.Join(externalModulesDir, moduleName, moduleVersion)

			err = dml.downloadModule(moduleName, moduleVersion, item.Labels["source"], moduleVersionPath)
			if err != nil {
				log.Warnf("Download module %q with version %s failed: %s. Skipping", moduleName, moduleVersion, err)
				continue
			}

			// restore symlink
			moduleRelativePath := filepath.Join("../", moduleName, moduleVersion)
			symlinkPath := filepath.Join(symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, moduleName))
			err = restoreModuleSymlink(externalModulesDir, symlinkPath, moduleRelativePath)
			if err != nil {
				log.Warnf("Create symlink for module %q failed: %s", moduleName, err)
				continue
			}

			fmt.Println("MODULE RESTORED", moduleName, moduleVersion)
		}
	}

	return nil

	// TODO: get all ModuleRelease with Deployed
	// TODO: check on file system
	// TODO: download if not compared
}

func (dml *DeckhouseModuleLoader) downloadModule(moduleName, moduleVersion, moduleSource, modulePath string) error {
	if moduleSource == "" {
		return nil
	}

	ms, err := dml.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), moduleSource, metav1.GetOptions{})
	if err != nil {
		return err
	}

	repo := ms.Spec.Registry.Repo

	opts := make([]cr.Option, 0)

	if ms.Spec.Registry.Scheme == "HTTP" {
		opts = append(opts, cr.WithInsecureSchema(true))
	}

	if ms.Spec.Registry.CA != "" {
		opts = append(opts, cr.WithCA(ms.Spec.Registry.CA))
	}

	if ms.Spec.Registry.DockerCFG != "" {
		opts = append(opts, cr.WithAuth(ms.Spec.Registry.DockerCFG))
	}

	regClient, err := cr.NewClient(path.Join(repo, moduleName), opts...)
	if err != nil {
		return err
	}

	img, err := regClient.Image(moduleVersion)
	if err != nil {
		return fmt.Errorf("fetch module version error: %v", err)
	}

	return copyModuleToFS(modulePath, img)
}

func copyModuleToFS(rootPath string, img regv1.Image) error {
	rc := mutate.Extract(img)
	defer rc.Close()

	err := copyLayersToFS(rootPath, rc)
	if err != nil {
		return fmt.Errorf("copy tar to fs: %w", err)
	}

	return nil
}

func copyLayersToFS(rootPath string, rc io.ReadCloser) error {
	if err := os.MkdirAll(rootPath, 0700); err != nil {
		return fmt.Errorf("mkdir root path: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar reader next: %w", err)
		}

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(rootPath, hdr.Name), 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(rootPath, hdr.Name))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("copy: %w", err)
			}
			outFile.Close()

			err = os.Chmod(outFile.Name(), os.FileMode(hdr.Mode)&0700) // remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err != nil {
				return fmt.Errorf("chmod: %w", err)
			}
		case tar.TypeSymlink:
			link := path.Join(rootPath, hdr.Name)
			if err := os.Symlink(hdr.Linkname, link); err != nil {
				return fmt.Errorf("create symlink: %w", err)
			}
		case tar.TypeLink:
			err := os.Link(path.Join(rootPath, hdr.Linkname), path.Join(rootPath, hdr.Name))
			if err != nil {
				return fmt.Errorf("create hardlink: %w", err)
			}

		default:
			return errors.New("unknown tar type")
		}
	}
}

func restoreModuleSymlink(externalModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}
