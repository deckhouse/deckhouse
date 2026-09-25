/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	configPath   = "/etc/keepalived-instance-config/config.json"
	authPassPath = "/etc/keepalived-instance-secret/authPass"
	outputPath   = "/etc/keepalived/keepalived.conf"
)

const keepalivedConfigTemplate = `
global_defs {
  preempt_delay 300
  advert_int 1
  vrrp_garp_master_refresh 30
}

%s
`

const vrrpInstanceTemplate = `
vrrp_instance %s {
  state %s
  interface %s
  virtual_router_id %d
  priority %d
  %s
  authentication {
    auth_type PASS
    auth_pass %s
  }
  virtual_ipaddress {
%s
  }
}
`

type config struct {
	Replicas      int            `json:"replicas"`
	VRRPInstances []vrrpInstance `json:"vrrpInstances"`
}

type vrrpInstance struct {
	ID                 int           `json:"id"`
	Interface          interfaceSpec `json:"interface"`
	Preempt            *bool         `json:"preempt"`
	VirtualIPAddresses []virtualIP   `json:"virtualIPAddresses"`
}

type interfaceSpec struct {
	DetectionStrategy string `json:"detectionStrategy"`
	Name              string `json:"name"`
	NetworkAddress    string `json:"networkAddress"`
}

type virtualIP struct {
	Address   string         `json:"address"`
	Interface *interfaceSpec `json:"interface"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	podNumber, err := podNumberFromEnv()
	if err != nil {
		return err
	}

	cfg, err := readConfig(configPath)
	if err != nil {
		return err
	}

	authPassBytes, err := os.ReadFile(authPassPath)
	if err != nil {
		return fmt.Errorf("could not read auth pass: %w", err)
	}
	authPass := string(authPassBytes)

	systemInterfaces, err := getSystemInterfaces()
	if err != nil {
		return fmt.Errorf("could not list system interfaces: %w", err)
	}

	var vrrpInstanceConfigs []string
	for index, instance := range cfg.VRRPInstances {
		rendered, err := renderVRRPInstance(instance, index, podNumber, cfg.Replicas, authPass, systemInterfaces)
		if err != nil {
			return err
		}
		vrrpInstanceConfigs = append(vrrpInstanceConfigs, rendered)
	}

	renderedConfig := fmt.Sprintf(keepalivedConfigTemplate, strings.Join(vrrpInstanceConfigs, "\n"))

	if err := os.WriteFile(outputPath, []byte(renderedConfig), 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", outputPath, err)
	}

	return nil
}

func podNumberFromEnv() (int, error) {
	podName := os.Getenv("POD_NAME")
	parts := strings.Split(podName, "-")
	podNumber, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, fmt.Errorf("could not parse pod number from POD_NAME %q: %w", podName, err)
	}
	return podNumber, nil
}

func readConfig(path string) (config, error) {
	var cfg config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("could not read config %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("could not parse config %s: %w", path, err)
	}

	return cfg, nil
}

func renderVRRPInstance(instance vrrpInstance, index, podNumber, replicas int, authPass string, systemInterfaces map[string][]string) (string, error) {
	name := "instance_" + strconv.Itoa(instance.ID)
	priority := (index+podNumber)%replicas + 1
	preempt := "preempt"
	if instance.Preempt != nil && !*instance.Preempt {
		preempt = "nopreempt"
	}

	iface, err := resolveInterface(instance.Interface, systemInterfaces)
	if err != nil {
		return "", fmt.Errorf("could not find interface for VRRP instance with ID '%d': %w", instance.ID, err)
	}

	var vipLines []string
	for _, vip := range instance.VirtualIPAddresses {
		vipIface := iface
		if vip.Interface != nil {
			vipIface, err = resolveInterface(*vip.Interface, systemInterfaces)
			if err != nil {
				return "", fmt.Errorf("could not find interface for virtual IP '%s' of VRRP instance with ID '%d': %w", vip.Address, instance.ID, err)
			}
		}
		vipLines = append(vipLines, "    "+vip.Address+" dev "+vipIface)
	}

	return fmt.Sprintf(vrrpInstanceTemplate,
		name,
		"BACKUP",
		iface,
		instance.ID,
		priority,
		preempt,
		authPass,
		strings.Join(vipLines, "\n"),
	), nil
}

func resolveInterface(spec interfaceSpec, systemInterfaces map[string][]string) (string, error) {
	var (
		iface string
		err   error
	)

	switch spec.DetectionStrategy {
	case "Name":
		iface = spec.Name
	case "NetworkAddress":
		iface, err = interfaceByNetwork(spec.NetworkAddress, systemInterfaces)
	case "DefaultRoute":
		iface, err = interfaceByDefaultRoute()
	default:
		return "", fmt.Errorf("unknown interface detectionStrategy %q", spec.DetectionStrategy)
	}
	if err != nil {
		return "", err
	}
	if iface == "" {
		return "", fmt.Errorf("no interface found for detectionStrategy %q", spec.DetectionStrategy)
	}

	return iface, nil
}

// getSystemInterfaces returns a map of interface name to the list of its IPv4 addresses in CIDR notation.
func getSystemInterfaces() (map[string][]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("could not list links: %w", err)
	}
	linkNameByIndex := make(map[int]string, len(links))
	for _, link := range links {
		attrs := link.Attrs()
		linkNameByIndex[attrs.Index] = attrs.Name
	}

	addrs, err := netlink.AddrList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("could not list addresses: %w", err)
	}

	ifaces := make(map[string][]string)
	for _, addr := range addrs {
		name, ok := linkNameByIndex[addr.LinkIndex]
		if !ok {
			continue
		}
		prefixLen, _ := addr.IPNet.Mask.Size()
		ifaces[name] = append(ifaces[name], fmt.Sprintf("%s/%d", addr.IPNet.IP.String(), prefixLen))
	}

	return ifaces, nil
}

// interfaceByNetwork returns the name of the system interface that has an address within the given network.
func interfaceByNetwork(network string, systemInterfaces map[string][]string) (string, error) {
	for ifname, systemNetworks := range systemInterfaces {
		for _, systemNetwork := range systemNetworks {
			ok, err := isNetworkInNetwork(systemNetwork, network)
			if err != nil {
				return "", err
			}
			if ok {
				return ifname, nil
			}
		}
	}
	return "", nil
}

// interfaceByDefaultRoute returns the name of the interface used by the default route in the main routing table.
func interfaceByDefaultRoute() (string, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: unix.RT_TABLE_MAIN}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return "", fmt.Errorf("could not list routes: %w", err)
	}
	for _, route := range routes {
		if route.Dst != nil {
			continue
		}
		link, err := netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			return "", fmt.Errorf("could not resolve link for default route: %w", err)
		}
		return link.Attrs().Name, nil
	}
	return "", fmt.Errorf("no default route found")
}

// isNetworkInNetwork reports whether net1's address falls within net2, comparing at the coarser of the two prefixes.
func isNetworkInNetwork(net1, net2 string) (bool, error) {
	net1Addr, net1Prefix, err := parseIPv4CIDR(net1)
	if err != nil {
		return false, err
	}
	net2Addr, net2Prefix, err := parseIPv4CIDR(net2)
	if err != nil {
		return false, err
	}

	net1Mask := uint32(0xFFFFFFFF) << (32 - net1Prefix)
	net2Mask := uint32(0xFFFFFFFF) << (32 - net2Prefix)

	return (net1Addr & net1Mask & net2Mask) == (net2Addr & net2Mask), nil
}

func parseIPv4CIDR(cidr string) (uint32, uint, error) {
	addrPart, prefixPart, found := strings.Cut(cidr, "/")
	if !found {
		return 0, 0, fmt.Errorf("invalid CIDR %q", cidr)
	}

	prefix, err := strconv.ParseUint(prefixPart, 10, 6)
	if err != nil || prefix > 32 {
		return 0, 0, fmt.Errorf("invalid CIDR prefix in %q", cidr)
	}

	octets := strings.Split(addrPart, ".")
	if len(octets) != 4 {
		return 0, 0, fmt.Errorf("invalid IPv4 address in %q", cidr)
	}
	var addrBytes [4]byte
	for i, octet := range octets {
		v, err := strconv.ParseUint(octet, 10, 8)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid IPv4 address in %q", cidr)
		}
		addrBytes[i] = byte(v)
	}

	return binary.BigEndian.Uint32(addrBytes[:]), uint(prefix), nil
}
