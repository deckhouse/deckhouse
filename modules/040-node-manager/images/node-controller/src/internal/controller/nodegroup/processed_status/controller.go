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

package processed_status

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	Client client.Client
}

// PatchProcessedStatus records the NodeGroup spec the controller has just processed into
// status.deckhouse. get_crds used to write status.deckhouse.observed BeforeHelm and the
// processed status AfterHelm, and status.deckhouse.synced tracked whether the two agreed.
// Node-controller observes and processes the spec in the same reconcile, so both checksums
// are the current one and synced is True once processed — without this writing observed too,
// synced (the Synced printer column) would stay False forever after the get_crds migration.
// The patch is skipped when nothing changed, so a reconcile triggered by an unrelated Node or
// Machine event does not churn a status write.
func (s *Service) PatchProcessedStatus(ctx context.Context, ngName string) error {
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "NodeGroup",
	})
	if err := s.Client.Get(ctx, types.NamespacedName{Name: ngName}, current); err != nil {
		return err
	}

	base := current.DeepCopy()
	filtered, err := ApplyNodeGroupCRDFilter(current)
	if err != nil {
		return fmt.Errorf("cannot apply filterFunc to object: %v", err)
	}

	filteredBytes, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("cannot marshal filtered object: %v", err)
	}
	objCheckSum := CalculateChecksum(string(filteredBytes))

	observedCheckSum, _, _ := unstructured.NestedString(current.Object, "status", "deckhouse", "observed", "checkSum")
	processedCheckSum, _, _ := unstructured.NestedString(current.Object, "status", "deckhouse", "processed", "checkSum")
	synced, _, _ := unstructured.NestedString(current.Object, "status", "deckhouse", "synced")
	if observedCheckSum == objCheckSum && processedCheckSum == objCheckSum && synced == "True" {
		return nil
	}

	if err := unstructured.SetNestedField(current.Object, "True", "status", "deckhouse", "synced"); err != nil {
		return fmt.Errorf("cannot set synced status field: %v", err)
	}
	timestamp := GetTimestamp()
	if err := unstructured.SetNestedStringMap(current.Object, map[string]string{
		"lastTimestamp": timestamp,
		"checkSum":      objCheckSum,
	}, "status", "deckhouse", "observed"); err != nil {
		return fmt.Errorf("cannot set observed status field: %v", err)
	}
	if err := unstructured.SetNestedStringMap(current.Object, map[string]string{
		"lastTimestamp": timestamp,
		"checkSum":      objCheckSum,
	}, "status", "deckhouse", "processed"); err != nil {
		return fmt.Errorf("cannot set processed status field: %v", err)
	}

	return s.Client.Status().Patch(ctx, current, client.MergeFrom(base))
}
