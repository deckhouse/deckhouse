package main

import (
	"fmt"
	"github.com/mholt/caddy"

	"flag"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/coredns/coredns/coremain"
	_ "github.com/coredns/coredns/plugin/bind"
	_ "github.com/coredns/coredns/plugin/cache"
	_ "github.com/coredns/coredns/plugin/errors"
	_ "github.com/coredns/coredns/plugin/forward"
	_ "github.com/coredns/coredns/plugin/health"
	_ "github.com/coredns/coredns/plugin/log"
	_ "github.com/coredns/coredns/plugin/loop"
	_ "github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	_ "github.com/coredns/coredns/plugin/reload"
	"k8s.io/dns/pkg/netif"
	"k8s.io/kubernetes/pkg/util/dbus"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"
)

// configParams lists the configuration options that can be provided to dns-cache
type configParams struct {
	localIP        string        // ip address for the local cache agent to listen for dns requests
	clusterDnsIP   string        // remote IP of cluster DNS service
	localPort      string        // port to listen for dns requests
	clusterDnsPort string        // remote port of cluster DNS server
	ifName         string        // Name of the interface to be created
	interval       time.Duration // specifies how often to run iptables rules check
	exitChan       chan bool     // Channel to terminate background goroutines
	kubeProxyMode	string // kube-proxy mode: ipvs or iptables
}

type iptablesChain struct {
	table utiliptables.Table
	chain utiliptables.Chain
}

type iptablesRule struct {
	table utiliptables.Table
	chain utiliptables.Chain
	args  []string
}

type cacheApp struct {
	iptables               utiliptables.Interface
	iptablesChains         []iptablesChain
	iptablesRulesToAppend  []iptablesRule
	iptablesRulesToPrepend []iptablesRule
	params                 configParams
	netifHandle            *netif.NetifManager
}

func (c *cacheApp) init() {
	var err error

	err = c.parseAndValidateFlags()
	if err != nil {
		clog.Fatalf("Error parsing flags: %s, exiting...", err)
	}

	c.netifHandle = netif.NewNetifManager(net.ParseIP(c.params.localIP))
	c.initIptables()

	// reinitialize everything just in case
	err = c.setupIptables()
	if err != nil {
		c.teardownNetworking()
		clog.Fatalf("Failed to setup iptables: %s, exiting...", err)
	}

	c.teardownNetworking()

	err = c.setupNetworking()
	if err != nil {
		c.teardownNetworking()
		clog.Fatalf("Failed to setup: %s, exiting", err)
	}
}

func (c *cacheApp) initIptables() {
	const nodeLocalDnsChain = utiliptables.Chain("NODE-LOCAL-DNS")

	c.iptablesChains = append(c.iptablesChains, iptablesChain{table: utiliptables.TableNAT, chain: nodeLocalDnsChain})

	if c.params.kubeProxyMode == "iptables" {
	c.iptablesRulesToPrepend = []iptablesRule{
		{utiliptables.TableNAT, utiliptables.ChainPrerouting, []string{"-d", c.params.localIP,
			"-p", "udp", "--dport", c.params.localPort, "-j", string(nodeLocalDnsChain)}},
		{utiliptables.TableNAT, utiliptables.ChainPrerouting, []string{"-d", c.params.localIP,
			"-p", "tcp", "--dport", c.params.localPort, "-j", string(nodeLocalDnsChain)}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-d", c.params.localIP, "-p", "udp", "-j", "KUBE-SVC-TCOU7JCQXEZGVUNU"}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-d", c.params.localIP, "-p", "tcp", "-j", "KUBE-SVC-ERIFXISQEP7F7OF4"}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-d", c.params.localIP, "-j", "KUBE-MARK-MASQ"}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-m", "socket", "-j", "RETURN"}},
	}}

	if c.params.kubeProxyMode == "ipvs" {c.iptablesRulesToPrepend = []iptablesRule{
		{utiliptables.TableNAT, utiliptables.ChainPrerouting, []string{"-d", c.params.localIP,
			"-p", "udp", "--dport", c.params.localPort, "-j", string(nodeLocalDnsChain)}},
		{utiliptables.TableNAT, utiliptables.ChainPrerouting, []string{"-d", c.params.localIP,
			"-p", "tcp", "--dport", c.params.localPort, "-j", string(nodeLocalDnsChain)}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-d", c.params.localIP, "-p", "udp", "--dport", c.params.localPort,
			"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%s", c.params.clusterDnsIP, c.params.clusterDnsPort)}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-d", c.params.localIP, "-p", "tcp", "--dport", c.params.localPort,
			"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%s", c.params.clusterDnsIP, c.params.clusterDnsPort)}},
		{utiliptables.TableNAT, nodeLocalDnsChain, []string{"-m", "socket", "-j", "RETURN"}},
	}}

	execer := utilexec.New()
	dbusInstance := dbus.New()
	c.iptables = utiliptables.New(execer, dbusInstance, utiliptables.ProtocolIpv4)
}

func (c cacheApp) setupIptables() error {
	var err error

	clog.Info("Setting up iptables...")

	for _, chain := range c.iptablesChains {
		_, err = c.iptables.EnsureChain(chain.table, chain.chain)
		if err != nil {
			return fmt.Errorf("failed to ensure existence of chain %q in table %q: %s",
				chain.chain, chain.table, err)
		}
	}

	c.flushIptablesChains()

	for _, rule := range c.iptablesRulesToPrepend {
		_, err = c.iptables.EnsureRule(utiliptables.Prepend, rule.table, rule.chain, rule.args...)
		if err != nil {
			return fmt.Errorf("failed to ensure existence of rule %v in table %q in chain %q: %s",
				rule.args, rule.table, rule.chain, err)
		}
	}

	for _, rule := range c.iptablesRulesToAppend {
		_, err = c.iptables.EnsureRule(utiliptables.Append, rule.table, rule.chain, rule.args...)
		if err != nil {
			return fmt.Errorf("failed to ensure existence of rule %v in table %q in chain %q: %s",
				rule.args, rule.table, rule.chain, err)
		}
	}

	return err
}

func (c cacheApp) flushIptablesChains() {
	for _, chain := range c.iptablesChains {
		if err := c.iptables.FlushChain(chain.table, chain.chain); err != nil {
			clog.Fatalf("Failed to flush chain %q in table %q: %s", chain.chain, chain.table, err)
		}
	}
}

func (c cacheApp) setupNetworking() error {
	var err error

	clog.Infof("Setting up dummy interface %q for node-local-dns", c.params.ifName)
	err = c.netifHandle.AddDummyDevice(c.params.ifName)
	if err != nil {
		return err
	}

	return err
}

func (c cacheApp) teardownNetworking() {
	clog.Infof("Tearing down dummy interface %q", c.params.ifName)
	if c.params.exitChan != nil {
		// Stop the goroutine that periodically checks for iptables rules/dummy interface
		// exitChan is a buffered channel of size 1, so this will not block
		c.params.exitChan <- true
	}

	err := c.netifHandle.RemoveDummyDevice(c.params.ifName)
	if err != nil {
		clog.Infof("Failed to tear down dummy interface %q: %s", c.params.ifName, err)
	}
}

func (c *cacheApp) parseAndValidateFlags() error {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Runs coreDNS v1.3.1 as a node-local-dns\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&c.params.localIP, "local-ip", "", "local ip address to bind to")
	flag.StringVar(&c.params.localPort, "local-port", "", "local port to bind node-local-dns to")
	flag.StringVar(&c.params.clusterDnsIP, "cluster-dns-ip", "", "cluster DNS ip")
	flag.StringVar(&c.params.clusterDnsPort, "cluster-dns-port", "", "cluster DNS port")
	flag.StringVar(&c.params.ifName, "ifname", "nodelocaldns", "name of the interface to be created")
	flag.StringVar(&c.params.kubeProxyMode, "kube-proxy-mode", "", "")
	flag.DurationVar(&c.params.interval, "sync-interval", 60, "interval (in seconds) to check for iptables rules")
	flag.Parse()

	if c.params.kubeProxyMode != "ipvs" && c.params.kubeProxyMode != "iptables" {
		return fmt.Errorf("invalid kube-proxy mode specified: %q", c.params.kubeProxyMode)
	}

	if net.ParseIP(c.params.localIP) == nil {
		return fmt.Errorf("invalid local-ip specified: %q", c.params.localIP)
	}
	if net.ParseIP(c.params.clusterDnsIP) == nil {
		return fmt.Errorf("invalid cluster DNS IP specified: %q", c.params.clusterDnsIP)
	}

	if _, err := strconv.Atoi(c.params.localPort); err != nil {
		return fmt.Errorf("invalid local port specified: %q", c.params.localPort)
	}
	if _, err := strconv.Atoi(c.params.clusterDnsPort); err != nil {
		return fmt.Errorf("invalid cluster DNS port specified: %q", c.params.clusterDnsPort)
	}
	return nil
}

func (c cacheApp) runChecks() {
	for _, chain := range c.iptablesChains {
		exists, err := c.iptables.EnsureChain(chain.table, chain.chain)
		if !exists {
			if err != nil {
				c.teardownNetworking()
				clog.Fatalf("Failed to add back non-existent chain %v: %s", chain, err)
			}
			clog.Infof("Added back non-existent chain %v", chain)
		}
		if err != nil {
			clog.Errorf("Failed to check existence of chain %v: %s", chain, err)
		}
	}

	for _, rule := range c.iptablesRulesToAppend {
		exists, err := c.iptables.EnsureRule(utiliptables.Prepend, rule.table, rule.chain, rule.args...)
		if !exists {
			if err != nil {
				c.teardownNetworking()
				clog.Fatalf("Failed to add back non-existent rule %v: %s", rule, err)
			}
			clog.Infof("Added back non-existent rule %v", rule)
		}
		if err != nil {
			clog.Errorf("Failed to check rule %v: %s", rule, err)
		}
	}

	for _, rule := range c.iptablesRulesToPrepend {
		exists, err := c.iptables.EnsureRule(utiliptables.Prepend, rule.table, rule.chain, rule.args...)
		if !exists {
			if err != nil {
				c.teardownNetworking()
				clog.Fatalf("Failed to add back non-existent rule %v: %s", rule, err)
			}
			clog.Infof("Added back non-existent rule %v", rule)
		}
		if err != nil {
			clog.Errorf("Failed to check rule %v: %s", rule, err)
		}
	}

	exists, err := c.netifHandle.EnsureDummyDevice(c.params.ifName)
	if !exists {
		if err != nil {
			c.teardownNetworking()
			clog.Fatalf("Failed to add back non-existent interface %q: %s", c.params.ifName, err)
		}
		clog.Infof("Added back nonexistent interface %s", c.params.ifName)
	}
	if err != nil {
		clog.Errorf("Failed to check dummy device %q: %s", c.params.ifName, err)
	}
}

func (c *cacheApp) run() {
	c.params.exitChan = make(chan bool, 1)
	tick := time.NewTicker(c.params.interval * time.Second)
	for {
		select {
		case <-tick.C:
			c.runChecks()
		case <-c.params.exitChan:
			clog.Warningf("Exiting iptables check goroutine")
			return
		}
	}
}

func main() {
	cache := new(cacheApp)
	cache.init()
	caddy.OnProcessExit = append(caddy.OnProcessExit, func() { cache.teardownNetworking() })
	// Ensure that the required setup is ready
	// https://github.com/kubernetes/dns/issues/282 sometimes the interface gets the ip and then loses it, if added too soon.
	cache.runChecks()
	go cache.run()
	coremain.Run()
	// Unlikely to reach here, if we did it is because coremain exited and the signal was not trapped.
	clog.Errorf("Untrapped signal, tearing down")
	cache.teardownNetworking()
}
