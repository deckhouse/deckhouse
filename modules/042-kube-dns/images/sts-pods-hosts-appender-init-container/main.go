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

	hostsEntry = `"%s	%s.%s.%s.svc.%s"`
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

	fd, err := os.Open(hostsFilePath)
	if err != nil {
		log.Fatal(err)
	}

	err = fd.Truncate(0)
	if err != nil {
		log.Fatalf("failed to truncate %q: %s", fd.Name(), err)
	}

	_, err = fd.WriteString(fmt.Sprintf(baseHosts, podIP, podHostname, podSubdomain, podNamespace, clusterDomain, podHostname))
	if err != nil {
		log.Fatal(err)
	}

	for _, alias := range strings.Fields(clusterDomainAliases) {
		_, err := fd.WriteString(fmt.Sprintf(hostsEntry, podIP, podHostname, podSubdomain, podNamespace, alias))
		if err != nil {
			log.Fatal(err)
		}
	}
}
