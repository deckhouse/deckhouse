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
	"encoding/base64"

	"k8s.io/apimachinery/pkg/conversion"
)

func Convert_v1alpha2_SSHCredentialsSpec_To_v1alpha1_SSHCredentialsSpec(in *v1alpha2.SSHCredentialsSpec, out *SSHCredentialsSpec, s conversion.Scope) error {
	staticinstancelog.Info("conversing from v1alpha2 to v1alpha1")
	decodedPass, err := base64.StdEncoding.DecodeString(in.SudoPasswordEncoded)
	if err != nil {
		return err
	}
	out.SudoPassword = string(decodedPass)
	return autoConvert_v1alpha2_SSHCredentialsSpec_To_v1alpha1_SSHCredentialsSpec(in, out, s)
}

func Convert_v1alpha1_SSHCredentialsSpec_To_v1alpha2_SSHCredentialsSpec(in *SSHCredentialsSpec, out *v1alpha2.SSHCredentialsSpec, s conversion.Scope) error {
	encodedPass := base64.StdEncoding.EncodeToString([]byte(in.SudoPassword))
	staticinstancelog.Info("conversing from v1alpha1 to v1alpha2")

	out.SudoPasswordEncoded = encodedPass
	return autoConvert_v1alpha1_SSHCredentialsSpec_To_v1alpha2_SSHCredentialsSpec(in, out, s)
}
