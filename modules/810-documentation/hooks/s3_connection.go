/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// s3ConnectionPath holds the keys of the Secret of the user's S3 storage
// (moduleStorage.s3.external.secretRef), base64-encoded as in the Secret, so
// the copy equals it byte for byte. The Secret may be in any namespace,
// and a pod mounts Secrets of its own namespace only: the chart renders a copy
// of it into d8-system (templates/s3-connection.yaml), so docs-builder needs
// no rights to read Secrets.
const s3ConnectionPath = "documentation.internal.s3Connection"

// s3ConnectionChecksumPath holds the checksum of the ConfigMap system-s3
// writes for the SystemBucket (moduleStorage.s3.type: System). docs-builder
// reads the connection once; the checksum in the pod template restarts the
// pod when system-s3 changes it, as the chart does not render it.
const (
	s3ConnectionChecksumPath = "documentation.internal.s3ConnectionChecksum"
	systemS3ConnectionName   = "documentation-s3-connection"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	// The Secret is read with a Get, not watched: the schedule bounds how long
	// a created or changed Secret takes to reach the pod.
	Schedule: []go_hook.ScheduleConfig{
		{Name: "s3_connection", Crontab: "*/5 * * * *"},
	},
}, dependency.WithExternalDependencies(copyS3Connection))

func copyS3Connection(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	s3Type := input.Values.Get("documentation.moduleStorage.s3.type").String()
	if s3Type != "External" {
		input.Values.Remove(s3ConnectionPath)
	}
	if s3Type != "System" {
		input.Values.Remove(s3ConnectionChecksumPath)
	}
	if s3Type != "External" && s3Type != "System" {
		return nil
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	if s3Type == "System" {
		cm, err := client.CoreV1().ConfigMaps("d8-system").Get(ctx, systemS3ConnectionName, metav1.GetOptions{})
		switch {
		case k8serrors.IsNotFound(err):
			// Once it appears, the checksum is set and the Deployment rolls
			// once (checksum/s3-connection in templates/deployment.yaml).
			input.Values.Remove(s3ConnectionChecksumPath)
		case err != nil:
			input.Logger.Warn("read the ConfigMap of the system S3 storage, the last checksum is kept", log.Err(err))
		default:
			input.Values.Set(s3ConnectionChecksumPath, checksum(cm.Data))
		}
		return nil
	}

	namespace := input.Values.Get("documentation.moduleStorage.s3.external.secretRef.namespace").String()
	name := input.Values.Get("documentation.moduleStorage.s3.external.secretRef.name").String()
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case k8serrors.IsNotFound(err):
		// Until it appears the versions are kept in the pod only.
		input.Values.Remove(s3ConnectionPath)
		return nil
	case err != nil:
		// The last copy stays: a failure of the API does not take S3 away.
		input.Logger.Warn("read the Secret of the S3 storage, the last copy is kept",
			slog.String("namespace", namespace), slog.String("name", name), log.Err(err))
		return nil
	}

	connection := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		connection[k] = base64.StdEncoding.EncodeToString(v)
	}
	input.Values.Set(s3ConnectionPath, connection)

	return nil
}

// checksum is the SHA-256 of the keys and values, in the order of the keys.
func checksum(data map[string]string) string {
	h := sha256.New()
	for _, k := range slices.Sorted(maps.Keys(data)) {
		fmt.Fprintf(h, "%s=%s\n", k, data[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}
