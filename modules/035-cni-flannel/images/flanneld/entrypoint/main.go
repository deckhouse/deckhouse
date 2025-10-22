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

package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	iptablesMgr, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Fatal(err)
	}

	err = insertUnique(iptablesMgr, "filter", "INPUT", strings.Fields("-m conntrack --ctstate INVALID -j DROP"), 1)
	if err != nil {
		log.Fatal(err)
	}

	if os.Getenv("POD_NETWORK_MODE") == "host-gw" {
		err := deleteLinksByPrefix("flannel")
		if err != nil {
			log.Fatal(err)
		}
	}

	var allIPs []string
	internalIPs, externalIPs, err := getInternalAndExternalIPs(hostname)
	if err != nil {
		log.Fatal(err)
	}
	allIPs = append(allIPs, internalIPs...)
	allIPs = append(allIPs, externalIPs...)
	if len(allIPs) == 0 {
		log.Fatalf("Both InternalIPs and ExternalIPs are empty for Node %q", hostname)
	}

	cniConfBytes, err := os.ReadFile("/etc/kube-flannel/cni-conf.json")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("/etc/cni/net.d/10-flannel.conflist", cniConfBytes, 0666)
	if err != nil {
		log.Fatal(err)
	}

	ciliumEnabledStr := os.Getenv("MODULE_CNI_CILIUM_ENABLED")
	if ciliumEnabledStr != "true" {
		err = os.Remove("/etc/cni/net.d/05-cilium.conflist")
		if err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}
	}

	var flannelArgs []string
	flannelArgs = append(flannelArgs, "flanneld")
	flannelArgs = append(flannelArgs, os.Args[1:]...)
	for _, ip := range allIPs {
		flannelArgs = append(flannelArgs, "-iface", ip)
	}
	err = unix.Exec("/opt/bin/flanneld", flannelArgs, os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}

func getInternalAndExternalIPs(nodeName string) (internalIPs []string, externalIPs []string, err error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodeObj, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	for _, ip := range nodeObj.Status.Addresses {
		switch ip.Type {
		case v1.NodeInternalIP:
			internalIPs = append(internalIPs, ip.Address)
		case v1.NodeExternalIP:
			externalIPs = append(externalIPs, ip.Address)
		}
	}

	return
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

func deleteLinksByPrefix(linkPrefix string) error {
	links, err := netlink.LinkList()
	if err != nil {
		return err
	}

	for _, link := range links {
		if strings.HasPrefix(link.Attrs().Name, linkPrefix) {
			err := netlink.LinkDel(link)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
