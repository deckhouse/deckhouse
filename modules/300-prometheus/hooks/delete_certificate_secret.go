package hooks

import (
    "strings"

    "github.com/flant/addon-operator/pkg/module_manager/go_hook"
    "github.com/flant/addon-operator/sdk"
    "github.com/flant/shell-operator/pkg/kube_events_manager/types"
    corev1 "k8s.io/api/core/v1"
    certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"    
)

type Secret struct {
	Name        string
	Annotations map[string]string
}


var _ = sdk.RegisterFunc(&go_hook.HookConfig{
    Queue: "/modules/prometheus/delete_certificate_secret",
    Kubernetes: []go_hook.KubernetesConfig{
        {
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
            FilterFunc: applySecretFilter,
            NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
		},
        {
            Name:       "certificates",
            ApiVersion: "cert-manager.io/v1",
            Kind:       "Certificate",
            FilterFunc: applyCertificateFilter,
            NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
        },
    },
}, handleCertificateDeletion)

func applyCertificateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
    var cert certv1.Certificate

    err := sdk.FromUnstructured(obj, &cert)
    if err != nil {
        return nil, err
    }

    return cert.Spec.SecretName, nil
}

func applySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	secrets := &Secret{
		Name:        secret.Name,
		Annotations: secret.Annotations,
	}

	return secrets, nil
}

func handleCertificateDeletion(input *go_hook.HookInput) error {
    certificates := input.Snapshots["certificates"]
    secrets := input.Snapshots["secrets"]

    certificateSecretNames := make(map[string]struct{})
    for _, item := range certificates {
        secretName := item.(string)
        certificateSecretNames[secretName] = struct{}{}
    }

    for _, item := range secrets {
        secret := item.(*Secret)
        secretName := secret.Name
        secretAnnotations := secret.Annotations

        if strings.HasPrefix(secretName, "ingress") {
            if _, found := certificateSecretNames[secretName]; !found {
                if altNames, exists := secretAnnotations["cert-manager.io/alt-names"]; exists {
                    altNamesList := strings.Split(altNames, ",")

                    for _, name := range altNamesList {
                        if strings.HasPrefix(name, "grafana") {
                            continue
                        }
                    }
                }

                input.PatchCollector.Delete("v1", "secrets", "d8-monitoring", secretName)
            }
        }
    }

    return nil
}


