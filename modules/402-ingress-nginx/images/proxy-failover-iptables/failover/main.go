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
	"io"
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

	natTable        = "nat"
	filterTable     = "filter"
	mangleTable     = "mangle"
	preroutingChain = "PREROUTING"
	inputChain      = "INPUT"
)

func main() {
	iptablesMgr, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Fatal(err)
	}

	migrationRemoveOldRules(iptablesMgr)

	err = addLinkAndAddress()
	if err != nil {
		log.Fatal(err)
	}

	// during the failover rollout remove failover-jump-rule setting all traffic to primary
	_ = iptablesMgr.DeleteIfExists(natTable, preroutingChain, jumpRule...)
	// do nothing on error, since ingress-failover chain may not exist yet

	// check 1081/1444 ports accepted
	err = insertUnique(iptablesMgr, filterTable, inputChain, inputAcceptRule, 1)
	if err != nil {
		log.Fatal(err)
	}

	// restore conn mark
	err = insertUnique(iptablesMgr, mangleTable, preroutingChain, restoreHttpMarkRule, 1)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, mangleTable, preroutingChain, restoreHttpsMarkRule, 2)
	if err != nil {
		log.Fatal(err)
	}

	// fill ingress-failover chain
	if exists, err := iptablesMgr.ChainExists(natTable, chainName); err != nil {
		log.Fatal(err)
	} else if !exists {
		err = iptablesMgr.NewChain(natTable, chainName)
		if err != nil {
			log.Fatal(err)
		}
	}
	err = insertUnique(iptablesMgr, natTable, chainName, socketExistsRule, 1)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, natTable, chainName, markHttpRule, 2)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, natTable, chainName, markHttpsRule, 3)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, natTable, chainName, saveMarkRule, 4)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, natTable, chainName, dnatHttpRule, 5)
	if err != nil {
		log.Fatal(err)
	}
	err = insertUnique(iptablesMgr, natTable, chainName, dnatHttpsRule, 6)
	if err != nil {
		log.Fatal(err)
	}

	// restore jump rule
	err = insertUnique(iptablesMgr, natTable, preroutingChain, jumpRule, 1)
	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signals:
			err = iptablesMgr.DeleteIfExists(natTable, preroutingChain, jumpRule...)
			if err != nil {
				log.Fatal(err)
			}
			err = iptablesMgr.ClearAndDeleteChain(natTable, chainName)
			if err != nil {
				log.Fatal(err)
			}

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
		return iptablesMgr.DeleteIfExists(natTable, chainName, socketExistsRule...)
	}

	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Got %d status code", resp.StatusCode)
		return iptablesMgr.DeleteIfExists(natTable, chainName, socketExistsRule...)
	}

	return insertUnique(iptablesMgr, natTable, chainName, socketExistsRule, 1)
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

// TODO: remove in 1.54 remove old rules after change ports 81->1081, 444->1444
func migrationRemoveOldRules(iptablesMgr *iptables.IPTables) {
	rules := []rulespec{
		{
			table: natTable,
			chain: chainName,
			rule:  strings.Fields("-p tcp --dport 80 -j DNAT --to-destination 169.254.20.11:81"),
		},
		{
			table: natTable,
			chain: chainName,
			rule:  strings.Fields("-p tcp --dport 443 -j DNAT --to-destination 169.254.20.11:444"),
		},
		{
			table: filterTable,
			chain: inputChain,
			rule:  strings.Fields("-p tcp -m multiport --dport 81,444 -d 169.254.20.11 -m comment --comment ingress-failover -j ACCEPT"),
		},
	}

	for _, rule := range rules {
		ok, err := iptablesMgr.Exists(rule.table, rule.chain, rule.rule...)
		if err != nil {
			log.Printf("migrationRemoveOldRules error: %v", err)
			continue
		}

		if !ok {
			continue
		}

		err = iptablesMgr.Delete(rule.table, rule.chain, rule.rule...)
		if err != nil {
			log.Printf("migrationRemoveOldRules error: %v", err)
		}
	}
}
