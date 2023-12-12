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
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

type iptablesRule struct {
	Table string
	Chain string
	Rule  []string
	Pos   int
}

func main() {
	network := "10.55.55.0/24"
	protocol := "tcp"
	mgmtport := "8989"
	routeTable := 10
	rulePriority := 1

	if len(os.Args) == 1 {
		log.Fatalln("Arg must be tcp or udp")
	}

	if os.Args[1] == "tcp" || os.Args[1] == "udp" {
		protocol = os.Args[1]
	} else {
		log.Fatalln("Arg must be tcp or udp: ", os.Args[1])
	}

	if protocol == "udp" {
		mgmtport = "9090"
		routeTable = 11
	}

	iptablesMgr, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Fatal(err)
	}

	nat := iptablesRule{
		Table: "nat",
		Chain: "POSTROUTING",
		Rule:  strings.Fields(fmt.Sprintf("-s %s ! -d %s -j MASQUERADE", network, network)),
		Pos:   1,
	}

	manglePreroutingSetMark := iptablesRule{
		Table: "mangle",
		Chain: "PREROUTING",
		Rule:  strings.Fields(fmt.Sprintf("-i tun-%s -j CONNMARK --set-mark %d", protocol, routeTable)),
		Pos:   1,
	}

	manglePreroutingRestoreMark := iptablesRule{
		Table: "mangle",
		Chain: "PREROUTING",
		Rule:  strings.Fields("! -i tun+ -j CONNMARK --restore-mark"),
		Pos:   2,
	}
	mangleOutputRestoreMark := iptablesRule{
		Table: "mangle",
		Chain: "OUTPUT",
		Rule:  strings.Fields("-j CONNMARK --restore-mark"),
		Pos:   1,
	}

	err = insertUnique(iptablesMgr, nat.Table, nat.Chain, nat.Rule, nat.Pos)
	if err != nil {
		log.Fatal(err)
	}

	err = insertUnique(iptablesMgr, manglePreroutingSetMark.Table, manglePreroutingSetMark.Chain, manglePreroutingSetMark.Rule, manglePreroutingSetMark.Pos)
	if err != nil {
		log.Fatal(err)
	}

	err = insertUnique(iptablesMgr, manglePreroutingRestoreMark.Table, manglePreroutingRestoreMark.Chain, manglePreroutingRestoreMark.Rule, manglePreroutingRestoreMark.Pos)
	if err != nil {
		log.Fatal(err)
	}

	err = insertUnique(iptablesMgr, mangleOutputRestoreMark.Table, mangleOutputRestoreMark.Chain, mangleOutputRestoreMark.Rule, mangleOutputRestoreMark.Pos)
	if err != nil {
		log.Fatal(err)
	}

	rule := netlink.NewRule()
	rule.Table = routeTable
	rule.Priority = rulePriority
	rule.Mark = routeTable
	err = netlink.RuleAdd(rule)

	if err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
	}

	err = mknodDevNetTun()
	if err != nil {
		log.Fatal(err)
	}

	var args []string
	args = append(args, "--config")
	args = append(args, "/home/v.snurnitsin/store/dev/go/exec-unix-args/openvpn.conf")
	args = append(args, fmt.Sprintf("--proto"))
	args = append(args, protocol)
	args = append(args, "--management")
	args = append(args, "127.0.0.1")
	args = append(args, mgmtport)
	args = append(args, "--dev")
	args = append(args, fmt.Sprintf("tun-%s", protocol))
	log.Println(args)

	cmd := exec.Command("/usr/sbin/openvpn", args...)
	cmd.Stdout = os.Stdout
	err = cmd.Start()
	if err != nil {
		log.Println(err)
	}
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

func mknodDevNetTun() error {
	_, err := os.Stat("/dev")
	if os.IsNotExist(err) {
		err := unix.Mount("dev", "/dev", "devtmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_RELATIME, "size=10m,nr_inodes=248418,mode=755")
		if err != nil {
			return fmt.Errorf("error mounting %s to %s: %v", "dev", "/dev", err)
		}
	}

	err = unix.Mkdir("/dev/net", 0753)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error create dir /dev/net: %v", err)
		}
	}

	path := "/dev/net/tun"
	var mode uint32 = 0666
	var major uint32 = 10
	var minor uint32 = 200
	dev := int(unix.Mkdev(major, minor))
	err = unix.Mknod(path, mode, dev)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error making device %s: %v", path, err)
		}
	}
	return nil
}

func netLinkCreateTuntap(name string, mtu int) {
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = name
	linkAttrs.MTU = mtu

	l := &netlink.Tuntap{
		LinkAttrs: linkAttrs,
		Mode:      unix.IFF_TUN,
	}
	err := netlink.LinkAdd(l)
	if err != nil {
		log.Println(err)
	}

	link, _ := netlink.LinkByName(linkAttrs.Name)
	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Println(err)
	}
}

func routeAdd(dst string, gw string, table int) {
	dstIPNet, err := parseIPNet(dst)
	if err != nil {
		fmt.Println("error parse IPNet: ", err)
	}

	route := netlink.Route{Gw: net.ParseIP(gw), Dst: dstIPNet, Table: table}
	err = netlink.RouteAdd(&route)
	if err != nil {
		fmt.Printf("add route: %s error: %s\n", route.String(), err.Error())
	}
}

func parseIPNet(address string) (*net.IPNet, error) {
	ip := net.ParseIP(address)
	if ip != nil {
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 8*net.IPv4len)}, nil
	}
	_, IPNet, err := net.ParseCIDR(address)
	if err != nil {
		return nil, err
	}
	return IPNet, nil
}
