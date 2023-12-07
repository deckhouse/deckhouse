// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/module"
)

const (
	leaseLabel    = "deckhouse.io/documentation-builder-sync"
	namespace     = "d8-system"
	resyncTimeout = time.Minute
)

func NewModuleDocsSyncer() (*ModuleDocsSyncer, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("get cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		resyncTimeout,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = leaseLabel
		}),
	)

	informer := factory.Coordination().V1().Leases().Informer()

	dClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("get dynamic client: %w", err)
	}

	httpClient := d8http.NewClient(d8http.WithTimeout(3 * time.Minute))
	return &ModuleDocsSyncer{dClient, informer, httpClient}, nil
}

type ModuleDocsSyncer struct {
	dClient    dynamic.Interface
	informer   cache.SharedIndexInformer
	httpClient d8http.Client
}

func (s *ModuleDocsSyncer) Run(ctx context.Context) {
	_, err := s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err := s.onLease(ctx)
			if err != nil {
				log.Error("module docs syncer: on lease:", err)
			}
		},
	})
	if err != nil {
		log.Error("add event handler:", err)
	}

	s.informer.Run(ctx.Done())
}

func (s *ModuleDocsSyncer) onLease(ctx context.Context) error {
	msGVR := schema.ParseGroupResource("modulesources.deckhouse.io").WithVersion("v1alpha1")
	list, err := s.dClient.Resource(msGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	for _, item := range list.Items {
		repo, _, _ := unstructured.NestedString(item.UnstructuredContent(), "spec", "registry", "repo")
		dockerCfg, _, _ := unstructured.NestedString(item.UnstructuredContent(), "spec", "registry", "dockerCfg")
		ca, _, _ := unstructured.NestedString(item.UnstructuredContent(), "spec", "registry", "ca")
		releaseChannel, _, _ := unstructured.NestedString(item.UnstructuredContent(), "spec", "releaseChannel")

		opts := make([]cr.Option, 0)
		if dockerCfg != "" {
			opts = append(opts, cr.WithAuth(dockerCfg))
		} else {
			opts = append(opts, cr.WithDisabledAuth())
		}

		if ca != "" {
			opts = append(opts, cr.WithCA(ca))
		}

		regCli, err := cr.NewClient(repo, opts...)
		if err != nil {
			return fmt.Errorf("get regestry client: %w", err)
		}

		tags, err := regCli.ListTags()
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		sort.Strings(tags)
		for _, moduleName := range tags {
			regCli, err := cr.NewClient(path.Join(repo, moduleName), opts...)
			if err != nil {
				return fmt.Errorf("fetch module %s: %v", moduleName, err)
			}

			moduleVersion, err := fetchModuleVersion(releaseChannel, repo, moduleName, opts)
			if err != nil {
				return fmt.Errorf("fetch module version: %w", err)
			}

			img, err := regCli.Image(moduleVersion)
			if err != nil {
				return fmt.Errorf("fetch module %s %s image: %v", moduleName, moduleVersion, err)
			}

			err = s.buildDocumentation(img, moduleName, moduleVersion)
			if err != nil {
				return fmt.Errorf("build documentation for %s %s: %w", moduleName, moduleVersion, err)
			}
		}
	}

	return nil
}

func fetchModuleVersion(releaseChannel, repo, moduleName string, registryOptions []cr.Option) (moduleVersion string, err error) {
	regCli, err := cr.NewClient(path.Join(repo, moduleName, "release"), registryOptions...)
	if err != nil {
		return "", fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return "", fmt.Errorf("fetch image error: %v", err)
	}

	moduleMetadata, err := fetchModuleReleaseMetadata(img)
	if err != nil {
		return "", fmt.Errorf("fetch release metadata error: %v", err)
	}

	return "v" + moduleMetadata.Version.String(), nil
}

type moduleReleaseMetadata struct {
	Version *semver.Version `json:"version"`
}

func fetchModuleReleaseMetadata(img v1.Image) (moduleReleaseMetadata, error) {
	buf := bytes.NewBuffer(nil)
	var meta moduleReleaseMetadata

	layers, err := img.Layers()
	if err != nil {
		return meta, err
	}

	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			// dcr.logger.Warnf("couldn't calculate layer size")
			return meta, err
		}
		if size == 0 {
			// skip some empty werf layers
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			return meta, err
		}

		err = untarMetadata(rc, buf)
		if err != nil {
			return meta, err
		}

		rc.Close()
	}

	err = json.Unmarshal(buf.Bytes(), &meta)

	return meta, err
}

func untarMetadata(rc io.ReadCloser, rw io.Writer) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		switch hdr.Name {
		case "version.json":
			_, err = io.Copy(rw, tr)
			if err != nil {
				return err
			}
			return nil

		default:
			continue
		}
	}
}

func (s *ModuleDocsSyncer) buildDocumentation(img v1.Image, moduleName, moduleVersion string) error {
	rc := module.ExtractDocs(img)
	defer rc.Close()

	const docsBuilderBasePath = "http://documentation-builder.d8-system.svc.cluster.local:8081"

	url := fmt.Sprintf("%s/loadDocArchive/%s/%s", docsBuilderBasePath, moduleName, moduleVersion)
	response, statusCode, err := s.httpPost(url, rc)
	if err != nil {
		return fmt.Errorf("POST %q return %d %q: %w", url, statusCode, response, err)
	}

	url = fmt.Sprintf("%s/build", docsBuilderBasePath)
	response, statusCode, err = s.httpPost(url, nil)
	if err != nil {
		return fmt.Errorf("POST %q return %d %q: %w", url, statusCode, response, err)
	}

	return nil
}

func (s *ModuleDocsSyncer) httpPost(url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, 0, err
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	dataBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	return dataBytes, res.StatusCode, nil
}
