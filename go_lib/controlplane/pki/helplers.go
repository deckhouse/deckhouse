package pki

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"k8s.io/apimachinery/pkg/util/sets"
	certutil "k8s.io/client-go/util/cert"
)

func pathForCert(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.crt", name))
}

func pathForKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.key", name))
}

// rsaKeySizeFromAlgorithmType takes a known RSA algorithm defined in the kubeadm API
// an returns its key size. For unknown types it returns 0. For an empty type it returns
// the default size of 2048.
func rsaKeySizeFromAlgorithmType(keyType constants.EncryptionAlgorithmType) int {
	switch keyType {
	case constants.EncryptionAlgorithmRSA2048, "":
		return 2048
	case constants.EncryptionAlgorithmRSA3072:
		return 3072
	case constants.EncryptionAlgorithmRSA4096:
		return 4096
	default:
		return 0
	}
}

// RemoveDuplicateAltNames removes duplicate items in altNames.
func RemoveDuplicateAltNames(altNames *certutil.AltNames) {
	if altNames == nil {
		return
	}

	if altNames.DNSNames != nil {
		altNames.DNSNames = sets.List(sets.New(altNames.DNSNames...))
	}

	ipsKeys := make(map[string]struct{})
	var ips []net.IP
	for _, one := range altNames.IPs {
		if _, ok := ipsKeys[one.String()]; !ok {
			ipsKeys[one.String()] = struct{}{}
			ips = append(ips, one)
		}
	}
	altNames.IPs = ips
}
