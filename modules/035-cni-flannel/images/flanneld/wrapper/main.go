/*
Copyright 2024 Flant JSC

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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

func runCNI(done chan<- error, pidChan chan<- int) {
	cmd := exec.Command("/entrypoint", os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := cmd.Start()
	if err != nil {
		done <- err
		return
	}

	pidChan <- cmd.Process.Pid
	log.Printf("CNI started with PID: %d", cmd.Process.Pid)

	done <- cmd.Wait()
}

func deleteInterfaces(interfaceNames []string) {
	for _, interfaceName := range interfaceNames {
		link, err := netlink.LinkByName(interfaceName)
		if err == nil {
			if err := netlink.LinkDel(link); err != nil {
				log.Printf("failed to delete interface %s: %v", link.Attrs().Name, err)
			} else {
				log.Printf("interface removed: %s", link.Attrs().Name)
			}
		}
	}
}

func deleteConfigFiles(configsDir string, cni string) {
	files, _ := os.ReadDir(configsDir)
	for _, file := range files {
		if strings.Contains(file.Name(), cni) {
			fullPath := filepath.Join(configsDir, file.Name())
			if err := os.Remove(fullPath); err != nil {
				log.Printf("failed to delete configuration file %s: %v", fullPath, err)
			} else {
				log.Printf("configuration file removed: %s", fullPath)
			}
		}
	}
}

func clearIptables(chainPrefixes []string) {
	tables := []string{"filter", "nat", "mangle", "raw"}
	standardChains := []string{"PREROUTING", "POSTROUTING", "INPUT", "FORWARD", "OUTPUT"}

	ipt, err := iptables.New()
	if err != nil {
		log.Printf("failed to initialize iptables: %v", err)
		return
	}

	for _, table := range tables {
		chains, err := ipt.ListChains(table)
		if err != nil {
			log.Printf("failed to list chains in table %s: %v", table, err)
			continue
		}

		for _, chain := range chains {
			for _, prefix := range chainPrefixes {
				if strings.HasPrefix(chain, prefix) {
					err = ipt.ClearChain(table, chain)
					if err != nil {
						log.Printf("could not clear chain %s in table %s: %v", chain, table, err)
					}
					err = ipt.DeleteChain(table, chain)
					if err != nil {
						log.Printf("could not delete chain %s in table %s: %v", chain, table, err)
					} else {
						log.Printf("chain %s in table %s deleted successfully", chain, table)
					}
					break
				}
			}
		}
	}

	for _, table := range tables {
		for _, chain := range standardChains {
			rules, err := ipt.List(table, chain)
			if err != nil {
				log.Printf("failed to list rules in chain %s in table %s: %v", chain, table, err)
				continue
			}
			for _, rule := range rules {
				for _, prefix := range chainPrefixes {
					if strings.Contains(rule, prefix) {
						err = ipt.Delete(table, chain, rule)
						if err != nil {
							log.Printf("failed to delete rule %s in chain %s in table %s: %v", rule, chain, table, err)
						}
					}
				}
			}
		}
	}
}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	pidChan := make(chan int, 1)
	cniDone := make(chan error, 1)
	go runCNI(cniDone, pidChan)

	select {
	case sig := <-sigCh:
		log.Printf("received signal: %v", sig)

		p, err := os.FindProcess(<-pidChan)
		if err == nil {
			if err := p.Signal(syscall.SIGTERM); err != nil {
				log.Printf("error sending SIGTERM to CNI: %v", err)
			}
		}

		select {
		case err := <-cniDone:
			if err != nil {
				log.Printf("CNI exited with error: %v", err)
			} else {
				log.Println("CNI exited gracefully")
			}
		case <-time.After(10 * time.Second):
			log.Println("timeout waiting for CNI to exit, killing it")
			if err := p.Kill(); err != nil {
				log.Printf("error killing CNI: %v", err)
			}
		}

	case err := <-cniDone:
		if err != nil {
			log.Printf("CNI exited with error: %v", err)
		} else {
			log.Println("CNI exited")
		}
	}

	log.Println("start system cleaning")
	deleteConfigFiles("/etc/cni/net.d/", "flannel")
	deleteInterfaces([]string{"cni0"})
	clearIptables([]string{
		"FLANNEL-",
		"CNI-",
	})
	clearIptables([]string{
		"KUBE-EXTERNAL-SERVICES",
		"KUBE-NODEPORTS",
		"KUBE-POSTROUTING",
		"KUBE-FORWARD",
		"KUBE-MARK-MASQ",
		"KUBE-PROXY-FIREWALL",
		"KUBE-SERVICES",
		"KUBE-PROXY-CANARY",
		"KUBE-SEP-",
		"KUBE-SVC-",
	})
}
