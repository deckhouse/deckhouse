package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/etcd"
)

const (
	moduleQueue = "/modules/control-plane-manager"
)

func getETCDClient(input *go_hook.HookInput, dc dependency.Container, endpoints []string) (etcd.Client, error) {
	ca := input.Values.Get("controlPlaneManager.internal.etcdCerts.ca").String()
	crt := input.Values.Get("controlPlaneManager.internal.etcdCerts.crt").String()
	key := input.Values.Get("controlPlaneManager.internal.etcdCerts.key").String()

	if ca == "" || crt == "" || key == "" {
		return nil, fmt.Errorf("etcd credentials not found")
	}

	caCert, clientCert, err := certificate.ParseCertificatesFromBase64(ca, crt, key)
	if err != nil {
		return nil, err
	}

	return dc.GetEtcdClient(endpoints, etcd.WithClientCert(clientCert, caCert), etcd.WithInsecureSkipVerify())
}
