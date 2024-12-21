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
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

const (
	wrapperAim = "/entrypoint"
)

func runCNI(done chan<- error, pidChan chan<- int) {
	cmd := exec.Command(wrapperAim, os.Args[1:]...)
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
	log.Printf("started clearing CNI prefixes: %s", strings.Join(chainPrefixes, ", "))

	cmd := exec.Command("/sbin/iptables-save")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("failed to get current iptables rules")
		return
	}

	restoreCommands := ""
	for _, line := range strings.Split(string(output), "\n") {
		prefixInLine := false
		for _, prefix := range chainPrefixes {
			if strings.Contains(line, prefix) {
				prefixInLine = true
				break
			}
		}
		if !prefixInLine {
			restoreCommands += fmt.Sprintln(line)
		}
	}

	tmpfile, err := os.CreateTemp("", "iptables-restore-")
	if err != nil {
		log.Printf("failed to create temporary file: %v", err)
		return
	}
	defer os.Remove(tmpfile.Name())

	if _, err = tmpfile.Write([]byte(restoreCommands)); err != nil {
		log.Printf("failed to write to temporary file: %v", err)
		return
	}
	if err = tmpfile.Close(); err != nil {
		log.Printf("failed to close temporary file: %v", err)
		return
	}

	cmd = exec.Command("/sbin/iptables-restore", tmpfile.Name())
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to clear iptables: %s\n%s", err, output)
	} else {
		log.Printf("iptables cleared successfully")
	}
}

func main() {
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

	// // Get CNI ModuleConfig
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
	// moduleStatus, found, err := unstructured.NestedBool(moduleConfig.Object, "spec", "enabled")
	// if err != nil {
	// 	log.Fatalf("Failed to get moduleStatus from ModuleConfig: %v", err)
	// }
	// if !found {
	// 	log.Fatalf("moduleStatus not found in ModuleConfig")
	// }
	// log.Printf("ModuleStatus: %t", moduleStatus)

	// // Get pods within podCIDR
	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	log.Fatalf("failed to create clientset: %v", err)
	// }
	// nodeObj, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	// if err != nil {
	// 	log.Fatalf("failed to get node %s: %v", nodeName, err)
	// }
	// podCIDR := nodeObj.Spec.PodCIDR
	// if podCIDR == "" {
	// 	log.Fatalf("PodCIDR is empty for node %s", nodeName)
	// }
	// log.Printf("node: %s, PodCIDR: %s\n", nodeName, podCIDR)

	// _, ipNet, err := net.ParseCIDR(podCIDR)
	// if err != nil {
	// 	log.Fatalf("failed to parse PodCIDR %s: %v", podCIDR, err)
	// }

	// pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
	// 	FieldSelector: "spec.nodeName=" + nodeName,
	// })
	// if err != nil {
	// 	log.Fatalf("failed to list pods on node %s: %v", nodeName, err)
	// }

	// for _, pod := range pods.Items {
	// 	podIP := net.ParseIP(pod.Status.PodIP)
	// 	if podIP == nil {
	// 		log.Printf("pod %s/%s has no IP yet, skipping", pod.Namespace, pod.Name)
	// 		continue
	// 	}

	// 	if ipNet.Contains(podIP) {
	// 		log.Printf(
	// 			"deleting pod %s/%s (IP: %s) because it belongs to PodCIDR %s\n",
	// 			pod.Namespace,
	// 			pod.Name,
	// 			pod.Status.PodIP,
	// 			podCIDR,
	// 		)

	// 		// gracePeriodSeconds := int64(30)
	// 		// deleteOptions := metav1.DeleteOptions{
	// 		// 	GracePeriodSeconds: &gracePeriodSeconds,
	// 		// }

	// 		// err := clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
	// 		// if err != nil {
	// 		// 	log.Printf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
	// 		// } else {
	// 		// 	log.Printf("pod %s/%s deleted successfully", pod.Namespace, pod.Name)
	// 		// }
	// 	}
	// }

	log.Println("started system cleaning")

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
