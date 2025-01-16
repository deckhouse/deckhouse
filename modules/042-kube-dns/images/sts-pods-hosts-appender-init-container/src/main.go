/*
Copyright 2023 Flant JSC

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

package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	hostsFilePath = "/mnt/hosts"
	baseHosts     = `
127.0.0.1	localhost
::1		localhost ip6-localhost ip6-loopback
fe00::0		ip6-localnet
fe00::0		ip6-mcastprefix
fe00::1		ip6-allnodes
fe00::2		ip6-allrouters
%s	%s.%s.%s.svc.%s	%s
`

	hostsEntry = `%s	%s.%s.%s.svc.%s`
)

func main() {
	var (
		podIP                = os.Getenv("POD_IP")
		podHostname          = os.Getenv("POD_HOSTNAME")
		podSubdomain         = os.Getenv("POD_SUBDOMAIN")
		podNamespace         = os.Getenv("POD_NAMESPACE")
		clusterDomain        = os.Getenv("CLUSTER_DOMAIN")
		clusterDomainAliases = os.Getenv("CLUSTER_DOMAIN_ALIASES")
	)

	fd, err := os.OpenFile(hostsFilePath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}

	_, err = fd.WriteString(fmt.Sprintf(baseHosts, podIP, podHostname, podSubdomain, podNamespace, clusterDomain, podHostname))
	if err != nil {
		log.Fatal(err)
	}

	for _, alias := range strings.Fields(clusterDomainAliases) {
		_, err := fd.WriteString(fmt.Sprintf(hostsEntry+"\n", podIP, podHostname, podSubdomain, podNamespace, alias))
		if err != nil {
			log.Fatal(err)
		}
	}
}
