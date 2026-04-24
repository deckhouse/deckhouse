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

package helper

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MigrationStateAnnotation = "ingress-nginx.deckhouse.io/migration-state"

	MigrationStateRunning  = "running"
	MigrationStateMigrated = "migrated"
)

func GetMigrationState(obj metav1.Object) string {
	if obj == nil || obj.GetAnnotations() == nil {
		return ""
	}

	return obj.GetAnnotations()[MigrationStateAnnotation]
}

func IsMigrated(obj metav1.Object) bool {
	return GetMigrationState(obj) == MigrationStateMigrated
}

func PatchMigrationState(
	ctx context.Context,
	kubeClient ctrlclient.Client,
	obj ctrlclient.Object,
	state string,
) error {
	current := GetMigrationState(obj)
	if current == state {
		return nil
	}

	base := obj.DeepCopyObject().(ctrlclient.Object)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[MigrationStateAnnotation] = state
	obj.SetAnnotations(annotations)

	return kubeClient.Patch(ctx, obj, ctrlclient.MergeFrom(base))
}
