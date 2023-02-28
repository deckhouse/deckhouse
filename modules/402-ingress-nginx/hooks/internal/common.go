/*
Copyright 2023 Flant JSC

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

package internal

import "github.com/flant/shell-operator/pkg/kube_events_manager/types"

var Namespace = "d8-ingress-nginx"

func NsSelector() *types.NamespaceSelector {
	return &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{Namespace},
		},
	}
}
