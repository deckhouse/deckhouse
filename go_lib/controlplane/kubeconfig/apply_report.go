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

package kubeconfig

// KubeconfigApplyReport describes outcomes of CreateKubeconfigFiles for each requested
// kubeconfig file. Entry order matches the order of the files slice passed to CreateKubeconfigFiles.
type KubeconfigApplyReport struct {
	Entries []KubeconfigApplyEntry
}

// KubeconfigApplyEntry is one row in KubeconfigApplyReport.
type KubeconfigApplyEntry struct {
	File   File
	Action KubeconfigEntryAction
}

// KubeconfigEntryAction describes what happened to the kubeconfig on disk.
type KubeconfigEntryAction uint8

const (
	// KubeconfigActionUnchanged means the existing file was kept (validation passed).
	KubeconfigActionUnchanged KubeconfigEntryAction = iota
	// KubeconfigActionWrittenCreated means the file was written and did not exist before.
	KubeconfigActionWrittenCreated
	// KubeconfigActionWrittenRegenerated means an existing file was replaced (validation failed).
	KubeconfigActionWrittenRegenerated
)

func (r *KubeconfigApplyReport) add(file File, action KubeconfigEntryAction) {
	r.Entries = append(r.Entries, KubeconfigApplyEntry{
		File:   file,
		Action: action,
	})
}
