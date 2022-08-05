package tls_certificate

import (
	"fmt"
	"strings"
)

const (
	publicDomainPrefix  = "%PUBLIC_DOMAIN%://"
	clusterDomainPrefix = "%CLUSTER_DOMAIN%://"
)

// PublicDomainSAN create template to enrich specified san with a public domain
func PublicDomainSAN(s string) string {
	return publicDomainPrefix + strings.TrimSuffix(s, ".")
}

// ClusterDomainSAN create template to enrich specified san with a cluster domain
func ClusterDomainSAN(san string) string {
	return clusterDomainPrefix + strings.TrimSuffix(san, ".")
}

func getClusterDomainSAN(sanValue, clusterDomain string) string {
	sanValue = strings.TrimPrefix(sanValue, clusterDomainPrefix)

	return fmt.Sprintf("%s.%s", sanValue, clusterDomain)
}

func getPublicDomainSAN(sanValue, publicDomain string) string {
	sanValue = strings.TrimPrefix(sanValue, publicDomainPrefix)

	return fmt.Sprintf(publicDomain, sanValue)
}
