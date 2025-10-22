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
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

var (
	chainName            = "ingress-failover"
	jumpRule             = strings.Fields("-p tcp -m multiport --dports 80,443 -m addrtype --dst-type LOCAL -j ingress-failover")
	socketExistsRule     = strings.Fields("-m socket --nowildcard -m mark ! --mark 3 -j RETURN")
	markHttpRule         = strings.Fields("-p tcp --dport 80 -j CONNMARK --set-mark 3")
	markHttpsRule        = strings.Fields("-p tcp --dport 443 -j CONNMARK --set-mark 3")
	saveMarkRule         = strings.Fields("-j CONNMARK --save-mark")
	restoreHttpMarkRule  = strings.Fields("-p tcp --dport 80 -j CONNMARK --restore-mark")
	restoreHttpsMarkRule = strings.Fields("-p tcp --dport 443 -j CONNMARK --restore-mark")
	dnatHttpRule         = strings.Fields("-p tcp --dport 80 -j DNAT --to-destination 169.254.20.11:1081")
	dnatHttpsRule        = strings.Fields("-p tcp --dport 443 -j DNAT --to-destination 169.254.20.11:1444")
	inputAcceptRule      = strings.Fields("-p tcp -m multiport --dport 1081,1444 -d 169.254.20.11 -m comment --comment ingress-failover -j ACCEPT")

	linkName = "ingressfailover"
)

func main() {
	iptablesMgr, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Fatal(err)
	}

	err = addLinkAndAddress()
	if err != nil {
		log.Fatal(err)
	}

	// during the failover rollout remove failover-jump-rule setting all traffic to primary
	_ = iptablesMgr.DeleteIfExists("nat", "PREROUTING", jumpRule...)
	// do nothing on error, since ingress-failover chain may not exist yet

	// check 1081/1444 ports accepted
	err = insertUnique(iptablesMgr, "filter", "INPUT", inputAcceptRule, 1)
	if err != nil {
		log.Fatal(err)
	}

	// restore conn mark
	err = insertUnique(iptablesMgr, "mangle", "PREROUTING", restoreHttpMarkRule, 1)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, "mangle", "PREROUTING", restoreHttpsMarkRule, 2)
	if err != nil {
		log.Fatal(err)
	}

	// fill ingress-failover chain
	if exists, err := iptablesMgr.ChainExists("nat", chainName); err != nil {
		log.Fatal(err)
	} else if !exists {
		err = iptablesMgr.NewChain("nat", chainName)
		if err != nil {
			log.Fatal(err)
		}
	}
	err = insertUnique(iptablesMgr, "nat", chainName, socketExistsRule, 1)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, "nat", chainName, markHttpRule, 2)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, "nat", chainName, markHttpsRule, 3)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, "nat", chainName, saveMarkRule, 4)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, "nat", chainName, dnatHttpRule, 5)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, "nat", chainName, dnatHttpsRule, 6)
	if err != nil {
		log.Fatal(err)
	}

	// restore jump rule
	err = insertUnique(iptablesMgr, "nat", "PREROUTING", jumpRule, 1)
	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigs:
			log.Printf("caught %s signal, terminating...", sig)
			return

		case <-ticker.C:
			err := loop(iptablesMgr)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func loop(iptablesMgr *iptables.IPTables) error {
	resp, err := http.Get("http://127.0.0.1:10254/healthz")
	if err != nil {
		log.Println(err)
		return iptablesMgr.DeleteIfExists("nat", chainName, socketExistsRule...)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Got %d status code", resp.StatusCode)
		return iptablesMgr.DeleteIfExists("nat", chainName, socketExistsRule...)
	}

	return insertUnique(iptablesMgr, "nat", chainName, socketExistsRule, 1)
}

func insertUnique(iptablesMgr *iptables.IPTables, table, chain string, rule []string, pos int) error {
	ok, err := iptablesMgr.Exists(table, chain, rule...)
	if err != nil {
		return err
	}
	if !ok {
		err := iptablesMgr.Insert(table, chain, pos, rule...)
		if err != nil {
			return err
		}
	}

	return nil
}

func addLinkAndAddress() error {
	link, err := netlink.LinkByName(linkName)
	if errors.As(err, &netlink.LinkNotFoundError{}) {
		linkAttrs := netlink.NewLinkAttrs()
		linkAttrs.Name = linkName
		dummyLink := &netlink.Dummy{LinkAttrs: linkAttrs}
		err := netlink.LinkAdd(dummyLink)
		if err != nil {
			return err
		}
		return addLinkAndAddress()
	} else if err != nil {
		return err
	}

	err = netlink.AddrReplace(link, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(169, 254, 20, 11),
			Mask: net.CIDRMask(32, 32),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

type rulespec struct {
	table string
	chain string
	rule  []string
}
