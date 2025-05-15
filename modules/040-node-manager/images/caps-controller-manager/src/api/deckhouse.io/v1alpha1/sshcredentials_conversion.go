/*
Copyright 2025 Flant JSC

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

package v1alpha1

import (
	"caps-controller-manager/api/deckhouse.io/v1alpha2"

	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts SSHCredentials to the Hub version (v1alpha2).
func (src *SSHCredentials) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.SSHCredentials)
	if err := Convert_v1alpha1_SSHCredentials_To_v1alpha2_SSHCredentials(src, dst, nil); err != nil {
		return err
	}

	restored := &v1alpha2.SSHCredentials{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	return nil
}

// ConvertFrom converts SSHCredentials from the Hub version (v1alpha2) to this version (v1alpha1).
func (dst *SSHCredentials) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.SSHCredentials)
	if err := Convert_v1alpha2_SSHCredentials_To_v1alpha1_SSHCredentials(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts SSHCredentialsList to the Hub version (v1alpha2).
func (src *SSHCredentialsList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.SSHCredentialsList)
	return Convert_v1alpha1_SSHCredentialsList_To_v1alpha2_SSHCredentialsList(src, dst, nil)
}

// ConvertFrom converts SSHCredentialsList from the Hub version (v1alpha2) to this version (v1alpha1).
func (dst *SSHCredentialsList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.SSHCredentialsList)
	return Convert_v1alpha2_SSHCredentialsList_To_v1alpha1_SSHCredentialsList(src, dst, nil)
}
