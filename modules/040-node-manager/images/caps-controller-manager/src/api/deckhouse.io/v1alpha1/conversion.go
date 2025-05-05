package v1alpha1

import (
	"caps-controller-manager/api/deckhouse.io/v1alpha2"
	"encoding/base64"

	"k8s.io/apimachinery/pkg/conversion"
)

func Convert_v1alpha2_SSHCredentialsSpec_To_v1alpha1_SSHCredentialsSpec(in *v1alpha2.SSHCredentialsSpec, out *SSHCredentialsSpec, s conversion.Scope) error {
	decodedPass, err := base64.StdEncoding.DecodeString(in.SudoPasswordEncoded)
	if err != nil {
		return err
	}
	out.SudoPassword = string(decodedPass)
	return autoConvert_v1alpha2_SSHCredentialsSpec_To_v1alpha1_SSHCredentialsSpec(in, out, s)
}

func Convert_v1alpha1_SSHCredentialsSpec_To_v1alpha2_SSHCredentialsSpec(in *SSHCredentialsSpec, out *v1alpha2.SSHCredentialsSpec, s conversion.Scope) error {
	encodedPass := base64.StdEncoding.EncodeToString([]byte(in.SudoPassword))

	out.SudoPasswordEncoded = string(encodedPass)
	return autoConvert_v1alpha1_SSHCredentialsSpec_To_v1alpha2_SSHCredentialsSpec(in, out, s)
}
