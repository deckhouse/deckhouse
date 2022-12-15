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

package hooks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
)

const (
	certificateSecretName = "user-authz-webhook"
)

var ErrSkip = fmt.Errorf("skipping")

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	BeforeHookCheck: func(input *go_hook.HookInput) bool {
		// Migrate the secret structure in any case
		err := dependency.WithExternalDependencies(migrateSecretStructure)(input)
		if err != nil {
			input.LogEntry.Errorf("migrating secret structure: %v", err)
			return false // skip hook
		}

		var (
			secretExists        = len(input.Snapshots[tls_certificate.SnapshotKey]) > 0
			multitenancyEnabled = input.Values.Get("userAuthz.enableMultiTenancy").Bool()
		)

		return secretExists || multitenancyEnabled
	},

	SANs: tls_certificate.DefaultSANs([]string{"127.0.0.1"}),
	CN:   "127.0.0.1",

	Namespace:            internal.Namespace,
	TLSSecretName:        certificateSecretName,
	FullValuesPathPrefix: "userAuthz.internal.webhookCertificate",
})

// Migration: prior to Deckhouse 1.43, the certificate was stored in these fields. The library
// expects another structure, so these webhook-* fields are not included in the snapshot.
//
//	webhook-server.crt
//	webhook-server.key
//	ca.crt
//
// We switch them to the standard structure:
//
//	tls.crt
//	tls.key
//	ca.crt
//
// TODO: (migration) remove in Deckhouse 1.44
func migrateSecretStructure(input *go_hook.HookInput, dc dependency.Container) error {
	input.Values.Get("userAuthz.enableMultiTenancy").Bool()

	klient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot get kubernetes client: %v", err)
	}

	secret, err := klient.CoreV1().Secrets(internal.Namespace).Get(context.TODO(), certificateSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			// Secret does not exist, nothing to migrate
			return nil
		}
		return fmt.Errorf("cannot get secret %s/%s: %v", internal.Namespace, certificateSecretName, err)
	}

	if secret.Data["tls.crt"] != nil {
		// Already migrated
		return nil
	}

	// After this, the tls certificate library will be able to handle the certificate
	secret.Data["tls.crt"] = secret.Data["webhook-server.crt"]
	secret.Data["tls.key"] = secret.Data["webhook-server.key"]

	_, err = klient.CoreV1().Secrets(internal.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cannot update secret %s/%s: %v", internal.Namespace, certificateSecretName, err)
	}

	// We have just migrated the secret, so we need to skip the hook to avoid wrong snapshot.
	return fmt.Errorf("skipping hook (secret %s/%s has been migrated)", internal.Namespace, certificateSecretName)
}
