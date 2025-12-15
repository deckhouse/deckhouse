package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"bashible-apiserver/pkg/template"
)

const ConfigurationChecksumAnnotation = "bashible.deckhouse.io/configuration-checksum"

func SetConfigurationChecksumAnnotation(ctx template.Context, ng string, meta *metav1.ObjectMeta) {
	if checksum, ok := ctx.GetConfigurationChecksum(ng); ok {
		if meta.Annotations == nil {
			meta.Annotations = map[string]string{}
		}
		meta.Annotations[ConfigurationChecksumAnnotation] = checksum
	}
}
