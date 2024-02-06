/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/kube-client/manifest/releaseutil"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

/*
TODO: Migration hook, remove in the Deckhouse 1.57
Adopt resources from the d8-system namespace to a project namespace
*/
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(migrateReleases))

func migrateReleases(input *go_hook.HookInput, dc dependency.Container) error {
	k8sCli, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	helmSecretList, err := k8sCli.CoreV1().Secrets("d8-system").List(context.TODO(), v1.ListOptions{LabelSelector: "name=multitenancy-manager,owner=helm,status=deployed"})
	if err != nil {
		return err
	}

	for _, secret := range helmSecretList.Items {
		var adoptedCount int

		data, ok := secret.Data["release"]
		if !ok {
			continue
		}
		rel, err := helm3DecodeRelease(string(data))
		if err != nil {
			input.LogEntry.Warnf("Cannot decode release %s/%s: %s", secret.Namespace, secret.Name, err)
			continue
		}

		for _, manifestData := range releaseutil.SplitManifests(rel.Manifest) {
			var obj releaseObject
			err = yaml.Unmarshal([]byte(manifestData), &obj)
			if err != nil {
				input.LogEntry.Warnf("Cannot decode manifest %s: %s", manifestData, err)
				continue
			}

			if obj.Kind == "" || obj.APIVersion == "" {
				continue
			}

			// don't adopt system objects
			if obj.Metadata.Namespace == internal.D8MultitenancyManager || obj.Metadata.Name == internal.D8MultitenancyManager {
				continue
			}

			releaseName := obj.Metadata.Namespace
			// global resources, like Namespace
			// Namespace name is equal to a project name
			if obj.Metadata.Namespace == "" {
				releaseName = obj.Metadata.Name
			}

			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]string{
						"meta.helm.sh/release-name":      releaseName,
						"meta.helm.sh/release-namespace": internal.D8MultitenancyManager,
					},
				},
			}

			input.PatchCollector.MergePatch(patch, obj.APIVersion, obj.Kind, obj.Metadata.Namespace, obj.Metadata.Name, object_patch.IgnoreMissingObject())
			adoptedCount++
		}

		// delete secret to prevent resources deletion
		// system resources will be recreated
		// project resources are adopted for a new release
		log.WithField("module", "multitenancy-manager").Infof("Adopted %d resources", adoptedCount)
		if adoptedCount > 0 {
			input.PatchCollector.Delete("v1", "Secret", secret.Namespace, secret.Name, object_patch.InForeground())
		}
	}

	return nil
}

type releaseObject struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

var magicGzip = []byte{0x1f, 0x8b, 0x08}

type release struct {
	Manifest string `json:"manifest,omitempty" protobuf:"bytes,5,opt,name=manifest,proto3"`
}

// Import this from helm3 lib - https://github.com/helm/helm/blob/49819b4ef782e80b0c7f78c30bd76b51ebb56dc8/pkg/storage/driver/util.go#L56
// helm3DecodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func helm3DecodeRelease(data string) (*release, error) {
	// base64 decode string
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}
