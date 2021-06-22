package certificate

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ApplyCaSelfSignedCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned ca secret to secret: %v", err)
	}

	return Authority{
		Key:  string(secret.Data["tls.key"]),
		Cert: string(secret.Data["tls.crt"]),
	}, nil
}

func GetOrCreateCa(input *go_hook.HookInput, snapshot, cn string) (*Authority, error) {
	var selfSignedCA Authority

	certs := input.Snapshots[snapshot]
	if len(certs) == 1 {
		var ok bool
		selfSignedCA, ok = certs[0].(Authority)
		if !ok {
			return nil, fmt.Errorf("cannot convert sefsigned certificate to certificate authority")
		}
	} else {
		var err error
		selfSignedCA, err = GenerateCA(input.LogEntry, cn)
		if err != nil {
			return nil, fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}
	}

	return &selfSignedCA, nil
}
