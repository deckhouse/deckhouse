/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/miekg/dns"
	"github.com/vishvananda/netlink"
)

const (
	healthUrl = "http://127.0.0.1:9225/health"
	metricUrl = "http://127.0.0.1:4224/metrics"

	dnsServer              = "169.254.20.10"
	kubeClusterDomainEnv   = "KUBE_CLUSTER_DOMAIN"
	kubeDNSSvcIPEnv        = "KUBE_DNS_SVC_IP"
	domainTemplate         = "kubernetes.default.svc.%s."
	deviceName             = "nodelocaldns"
	shouldSetupIPtablesEnv = "SHOULD_SETUP_IPTABLES"

	readinessFilePath = "/tmp/coredns-readiness"
	readyState        = "ready"
	notReadyState     = "not-ready"
)

var (
	requestTimeout = time.Duration(1 * time.Second)
)

func main() {
	actionPtr := flag.String("action", "", "must be specified one of the following actions: start, readiness, liveness")
	flag.Parse()

	var err error
	switch *actionPtr {
	case "start":
		err = start()

	case "readiness":
		status := readyState
		err = livenessCheck()
		if err != nil {
			status = notReadyState
		}

		if writeErr := os.WriteFile(readinessFilePath, []byte(status), 0644); writeErr != nil {
			err = writeErr
		}

	case "liveness":
		err = livenessCheck()

	case "":
		fmt.Println("No action specified")
		os.Exit(1)

	default:
		fmt.Println("Unsupported action")
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("exit with error: %v", err)
		os.Exit(1)
	}
}

func doHttpRequest(client *http.Client, url string) error {
	ctx, _ := context.WithTimeout(context.Background(), requestTimeout)
	req, err := http.NewRequestWithContext(ctx, "GET", healthUrl, nil)
	if err != nil {
		return err
	}

	_, err = client.Do(req)

	return err
}

func doDnsRequest(domain, dnsServer string) error {
	c := new(dns.Client)
	c.Timeout = requestTimeout

	m := new(dns.Msg)
	m.SetQuestion(domain, dns.TypeA)
	m.RecursionDesired = true

	retries := 2

	for {
		r, _, err := c.Exchange(m, net.JoinHostPort(dnsServer, "53"))
		if err != nil {
			return err
		}

		if r.Rcode != dns.RcodeSuccess {
			retries--
			if retries > 0 {
				continue
			}

			return fmt.Errorf("failed to get A record for %s domain: retries exceeded", domain)
		}

		for _, a := range r.Answer {
			if _, ok := a.(*dns.A); ok {
				return nil
			}
		}

	}

	return fmt.Errorf("failed to get A record for %s domain", domain)
}

func livenessCheck() error {
	client := &http.Client{
		Timeout: requestTimeout,
	}

	if err := doHttpRequest(client, healthUrl); err != nil {
		return err
	}

	if err := doHttpRequest(client, metricUrl); err != nil {
		return err
	}

	clusterDomain := os.Getenv(kubeClusterDomainEnv)
	if len(clusterDomain) == 0 {
		return fmt.Errorf("%s is not set", kubeClusterDomainEnv)
	}

	domain := fmt.Sprintf(domainTemplate, clusterDomain)

	return doDnsRequest(domain, dnsServer)
}

func setupInterface(kubeDNSSvcIP string) error {
	link, err := netlink.LinkByName(deviceName)
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			linkAttrs := netlink.NewLinkAttrs()
			linkAttrs.Name = deviceName
			dummyLink := &netlink.Dummy{LinkAttrs: linkAttrs}
			err := netlink.LinkAdd(dummyLink)
			if err != nil {
				return err
			}

			if link, err = netlink.LinkByName(deviceName); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	dnsServerLinkLocalIP := net.ParseIP(dnsServer)
	dnsSvcIP := net.ParseIP(kubeDNSSvcIP)

	if !slices.ContainsFunc(addrs, func(a netlink.Addr) bool {
		return a.IP.Equal(dnsServerLinkLocalIP)
	}) {
		if err = netlink.AddrAdd(link, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   dnsServerLinkLocalIP,
				Mask: net.CIDRMask(32, 32),
			},
		}); err != nil {
			return err
		}
	}

	if !slices.ContainsFunc(addrs, func(a netlink.Addr) bool {
		return a.IP.Equal(dnsSvcIP)
	}) {
		if err = netlink.AddrAdd(link, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   dnsSvcIP,
				Mask: net.CIDRMask(32, 32),
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

func appendIPtablesRule(mgr *iptables.IPTables, iptableName, chainName string, rule []string) error {
	ok, err := mgr.Exists(iptableName, chainName, rule...)
	if err != nil {
		return err
	}

	if !ok {
		if err := mgr.Append(iptableName, chainName, rule...); err != nil {
			return err
		}
	}

	return nil
}

func deleteIPtablesRule(mgr *iptables.IPTables, iptableName, chainName string, rule []string) error {
	ok, err := mgr.Exists(iptableName, chainName, rule...)
	if err != nil {
		return err
	}

	if ok {
		if err := mgr.Delete(iptableName, chainName, rule...); err != nil {
			return err
		}
	}

	return nil
}

func setupIPtables(kubeDNSSvcIp string) error {
	iptablesMgr, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(60))
	if err != nil {
		return fmt.Errorf("failed to init iptables manager: %w", err)
	}

	dnsTCPRule := strings.Fields(fmt.Sprintf("-s %s/32 -p tcp -m tcp --sport 53 -j NOTRACK", kubeDNSSvcIp))
	if err := appendIPtablesRule(iptablesMgr, "raw", "OUTPUT", dnsTCPRule); err != nil {
		return fmt.Errorf("failed to append iptables tcp rules: %w", err)
	}

	dnsUDPRule := strings.Fields(fmt.Sprintf("-s %s/32 -p udp -m udp --sport 53 -j NOTRACK", kubeDNSSvcIp))
	if err := appendIPtablesRule(iptablesMgr, "raw", "OUTPUT", dnsUDPRule); err != nil {
		return fmt.Errorf("failed to append iptables udp rules: %w", err)
	}

	socketRule := strings.Fields(fmt.Sprintf("-d %s/32 -m socket --nowildcard -j NOTRACK", kubeDNSSvcIp))
	if err := deleteIPtablesRule(iptablesMgr, "raw", "OUTPUT", socketRule); err != nil {
		return fmt.Errorf("failed to delete iptables rules: %w", err)
	}

	return nil
}

func start() error {
	if os.Getenv(shouldSetupIPtablesEnv) == "yes" {
		kubeDNSSvcIP := os.Getenv(kubeDNSSvcIPEnv)
		if len(kubeDNSSvcIP) == 0 {
			return fmt.Errorf("%s env is not set", kubeDNSSvcIPEnv)
		}

		if err := setupInterface(kubeDNSSvcIP); err != nil {
			return err
		}

		if err := setupIPtables(kubeDNSSvcIP); err != nil {
			return err
		}

		if err := os.WriteFile(readinessFilePath, []byte(notReadyState), 0644); err != nil {
			return err
		}
	}

	if err := syscall.Exec("/coredns", []string{"/coredns", "-conf", "/etc/coredns/Corefile"}, os.Environ()); err != nil {
		return fmt.Errorf("failed to start coredns: %v", err)
	}

	return nil
}
