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
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

func execIptables(args ...string) (string, error) {
	args = append(args, "--wait")
	cmd := exec.Command("/sbin/iptables", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func clearIptables(chainPrefixes []string) {
	tables := []string{"filter", "nat", "mangle", "raw"}
	standardChains := []string{"PREROUTING", "POSTROUTING", "INPUT", "FORWARD", "OUTPUT"}

	log.Printf("clear CNI prefixes: %s", strings.Join(chainPrefixes, ", "))

	for _, table := range tables {
		for _, standardChain := range standardChains {
			output, err := execIptables("-t", table, "-L", standardChain, "--line-numbers")
			if err != nil {
				continue
			}
			log.Printf("table %s: clear rules in chain %s", table, standardChain)
			lines := strings.Split(strings.TrimSpace(output), "\n")
			for i := len(lines) - 1; i >= 0; i-- {
				for _, chain := range chainPrefixes {
					if strings.Contains(lines[i], chain) {
						lineSlice := strings.Fields(lines[i])
						number := lineSlice[0]
						out, err := execIptables("-t", table, "-D", standardChain, number)
						if err != nil {
							log.Printf("table %s: %s", table, out)
						}
					}
				}
			}
		}
	}

	for _, table := range tables {
		output, err := execIptables("-t", table, "-S")
		if err != nil {
			continue
		}

		log.Printf("table %s: clear CNI chains", table)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, rule := range lines {
			for _, chain := range chainPrefixes {
				if strings.HasPrefix(rule, "-N") && strings.Contains(rule, chain) {
					commandTail := strings.TrimPrefix(rule, "-N ")
					out, err := execIptables("-t", table, "-F", commandTail)
					if err != nil {
						log.Printf("table %s: %s", table, out)
					}
				}
			}
		}
		log.Printf("table %s: delete CNI chains", table)
		lines = strings.Split(strings.TrimSpace(output), "\n")
		for _, rule := range lines {
			for _, chain := range chainPrefixes {
				if strings.HasPrefix(rule, "-N") && strings.Contains(rule, chain) {
					commandTail := strings.TrimPrefix(rule, "-N ")
					out, err := execIptables("-t", table, "-X", commandTail)
					if err != nil {
						log.Printf("table %s: %s", table, out)
					}
				}
			}
		}
	}
}

func main() {
	moduleConfigName := "cni-flannel"

	nodeName, err := os.Hostname()
	if err != nil {
		log.Fatal(err) // FIXME: ???
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to get in-cluster config: %v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create dynamic client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	moduleConfigGVR := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "moduleconfigs",
	}
	moduleConfig, err := dynamicClient.Resource(moduleConfigGVR).Get(ctx, moduleConfigName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("failed to get ModuleConfig %s: %v", moduleConfigName, err)
	}
	moduleStatus, found, err := unstructured.NestedString(moduleConfig.Object, "spec", "enabled")
	if err != nil {
		log.Fatalf("Failed to get moduleStatus from ModuleConfig: %v", err)
	}
	if !found {
		log.Fatalf("moduleStatus not found in ModuleConfig")
	}
	log.Printf("ModuleStatus: %s", moduleStatus)

	// Handle CNI process
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
		case <-time.After(2 * time.Second):
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

	// Get CNI ModuleConfig
	// moduleConfigName := "cni-flannel"

	// nodeName, err := os.Hostname()
	// if err != nil {
	// 	log.Fatal(err) // FIXME: ???
	// }
	// config, err := rest.InClusterConfig()
	// if err != nil {
	// 	log.Fatalf("failed to get in-cluster config: %v", err)
	// }
	// dynamicClient, err := dynamic.NewForConfig(config)
	// if err != nil {
	// 	log.Fatalf("Failed to create dynamic client: %v", err)
	// }
	// ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	// defer cancel()

	// moduleConfigGVR := schema.GroupVersionResource{
	// 	Group:    "deckhouse.io",
	// 	Version:  "v1alpha1",
	// 	Resource: "moduleconfigs",
	// }
	// moduleConfig, err := dynamicClient.Resource(moduleConfigGVR).Get(ctx, moduleConfigName, metav1.GetOptions{})
	// if err != nil {
	// 	log.Fatalf("failed to get ModuleConfig %s: %v", moduleConfigName, err)
	// }
	// moduleStatus, found, err := unstructured.NestedString(moduleConfig.Object, "spec", "enabled")
	// if err != nil {
	// 	log.Fatalf("Failed to get moduleStatus from ModuleConfig: %v", err)
	// }
	// if !found {
	// 	log.Fatalf("moduleStatus not found in ModuleConfig")
	// }
	// log.Printf("ModuleStatus: %s", moduleStatus)

	// Get pods within podCIDR
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create clientset: %v", err)
	}
	nodeObj, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("failed to get node %s: %v", nodeName, err)
	}
	podCIDR := nodeObj.Spec.PodCIDR
	if podCIDR == "" {
		log.Fatalf("PodCIDR is empty for node %s", nodeName)
	}
	log.Printf("node: %s, PodCIDR: %s\n", nodeName, podCIDR)

	_, ipNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		log.Fatalf("failed to parse PodCIDR %s: %v", podCIDR, err)
	}

	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		log.Fatalf("failed to list pods on node %s: %v", nodeName, err)
	}

	for _, pod := range pods.Items {
		podIP := net.ParseIP(pod.Status.PodIP)
		if podIP == nil {
			log.Printf("pod %s/%s has no IP yet, skipping", pod.Namespace, pod.Name)
			continue
		}

		if ipNet.Contains(podIP) {
			log.Printf(
				"deleting pod %s/%s (IP: %s) because it belongs to PodCIDR %s\n",
				pod.Namespace,
				pod.Name,
				pod.Status.PodIP,
				podCIDR,
			)

			// gracePeriodSeconds := int64(30)
			// deleteOptions := metav1.DeleteOptions{
			// 	GracePeriodSeconds: &gracePeriodSeconds,
			// }

			// err := clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
			// if err != nil {
			// 	log.Printf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
			// } else {
			// 	log.Printf("pod %s/%s deleted successfully", pod.Namespace, pod.Name)
			// }
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
		"KUBE-SVC-",
		"KUBE-SEP-",
	})

	log.Println("finished system cleaning")
}
