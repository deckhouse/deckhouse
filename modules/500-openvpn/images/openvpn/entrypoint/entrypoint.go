package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
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

	var openvpnArgs []string
	openvpnArgs = append(openvpnArgs, "--config /etc/openvpn/openvpn.conf")
	//openvpnArgs = append(openvpnArgs, "/etc/openvpn/openvpn.conf")
	openvpnArgs = append(openvpnArgs, fmt.Sprintf("--proto %s", protocol))
	//openvpnArgs = append(openvpnArgs, protocol)
	openvpnArgs = append(openvpnArgs, fmt.Sprintf("--management 127.0.0.1 %s", mgmtport))
	//openvpnArgs = append(openvpnArgs, "127.0.0.1")
	//openvpnArgs = append(openvpnArgs, mgmtport)
	//openvpnArgs = append(openvpnArgs, "--dev")
	openvpnArgs = append(openvpnArgs, fmt.Sprintf("--dev tun-%s", protocol))

	err = unix.Exec("/openvpn", openvpnArgs, os.Environ())
	if err != nil {
		log.Fatal(err)
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
