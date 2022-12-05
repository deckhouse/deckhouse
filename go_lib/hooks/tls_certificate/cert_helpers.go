/*
Copyright 2022 Flant JSC

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
