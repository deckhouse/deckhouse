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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const emptyPasswordHashWarning = "Password hash is empty. This may not be secure and it may be prohibited by PAM settings."

var nodeUserListGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeUserList"}

// NodeUserValidator enforces uid uniqueness across NodeUsers with overlapping
// nodeGroups (reimplementation of the shell hook
// modules/040-node-manager/webhooks/validating/node_user).
type NodeUserValidator struct {
	Reader client.Reader
}

type existingNodeUser struct {
	name       string
	uid        int64
	nodeGroups []string
}

func (w *NodeUserValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var user struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			UID          int64    `json:"uid"`
			NodeGroups   []string `json:"nodeGroups"`
			PasswordHash string   `json:"passwordHash"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(req.Object.Raw, &user); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("parse NodeUser object: %w", err))
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(nodeUserListGVK)
	if err := w.Reader.List(ctx, list); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("list NodeUsers: %w", err))
	}

	others := make([]existingNodeUser, 0, len(list.Items))
	for _, item := range list.Items {
		if item.GetName() == user.Metadata.Name {
			continue
		}
		uid, _, err := unstructured.NestedInt64(item.Object, "spec", "uid")
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("read spec.uid of NodeUser %s: %w", item.GetName(), err))
		}
		nodeGroups, _, err := unstructured.NestedStringSlice(item.Object, "spec", "nodeGroups")
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("read spec.nodeGroups of NodeUser %s: %w", item.GetName(), err))
		}
		others = append(others, existingNodeUser{name: item.GetName(), uid: uid, nodeGroups: nodeGroups})
	}

	if msg := nodeUserDenyMessage(user.Spec.UID, user.Spec.NodeGroups, others); msg != "" {
		return admission.Denied(msg)
	}

	if user.Spec.PasswordHash == "" {
		return admission.Allowed("").WithWarnings(emptyPasswordHashWarning)
	}
	return admission.Allowed("")
}

// nodeUserDenyMessage returns a denial message if the uid is already taken by another
// NodeUser whose nodeGroups overlap with the given ones, or "" when there is no conflict.
func nodeUserDenyMessage(uid int64, nodeGroups []string, others []existingNodeUser) string {
	for _, other := range others {
		if other.uid == uid && slices.Contains(other.nodeGroups, "*") {
			return fmt.Sprintf(`The user with the uid: %d already exists in the nodeGroup: "*"`, uid)
		}
	}

	for _, nodeGroup := range nodeGroups {
		for _, other := range others {
			if other.uid != uid {
				continue
			}
			if nodeGroup == "*" {
				return fmt.Sprintf("The user with the uid: %d already exists in the nodeGroup: *", uid)
			}
			if slices.Contains(other.nodeGroups, nodeGroup) {
				return fmt.Sprintf("The user with the uid: %d already exists in the nodeGroup: %s", uid, nodeGroup)
			}
		}
	}

	return ""
}
