/*
Copyright 2021 Flant JSC

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

package vrl

// CleanUpAfterSourceRule is a general cleanup rule to sanitize the final message.
// It should always be the first rule in the transforms chain to avoid unexpected data leaks to transform rules.
const CleanUpAfterSourceRule Rule = `
if exists(.pod_labels."controller-revision-hash") {
    del(.pod_labels."controller-revision-hash")
}
if exists(.pod_labels."pod-template-hash") {
    del(.pod_labels."pod-template-hash")
}
if exists(.kubernetes) {
    del(.kubernetes)
}
if exists(.file) {
    del(.file)
}
if exists(.node_labels."node.deckhouse.io/group") {
	.node_group = (.node_labels."node.deckhouse.io/group")
}
del(.node_labels)
`
