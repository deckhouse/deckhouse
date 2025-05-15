/*
Copyright 2025 Flant JSC

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
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	chainName            = "ingress-failover"
	jumpRule             = strings.Fields("-p tcp -m multiport --dports 80,443 -m addrtype --dst-type LOCAL -j ingress-failover")
	restoreHttpMarkRule  = strings.Fields("-p tcp --dport 80 -j CONNMARK --restore-mark")
	restoreHttpsMarkRule = strings.Fields("-p tcp --dport 443 -j CONNMARK --restore-mark")
	inputAcceptRule      = strings.Fields("-p tcp -m multiport --dport 1081,1444 -d 169.254.20.11 -m comment --comment ingress-failover -j ACCEPT")
	linkName             = "ingressfailover"
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// In-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}

	// Clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	iptablesMgr, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		log.Fatal(err)
	}

	// Catch signals
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatal("NODE_NAME env variable is required")
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			log.Println("Shutting down by signal...")
			cancel()
			return

		case <-ticker.C:
			// Check if node has label with-failover-node=false
			shouldClean, err := HasFailoverLabelOnNode(ctx, clientset, nodeName)
			if err != nil {
				log.Printf("Error checking failover label: %v", err)
				continue
			}

			if shouldClean {
				log.Printf("Label is false. Cleaning up iptables on node %s", nodeName)
				if err := cleanup(iptablesMgr); err != nil {
					log.Printf("Failed to clean iptables: %v", err)
					continue
				}

				log.Printf("iptables cleaned. Removing label from node %s", nodeName)
				if err := RemoveFailoverLabel(ctx, clientset, nodeName); err != nil {
					log.Printf("Failed to remove label: %v", err)
					continue
				}

				log.Println("Label removed. Waiting for SIGTERM to exit gracefully...")
				ticker.Stop()

				// Waiting SIGTERM signal
				<-stopCh
				cancel()
				log.Println("Received shutdown signal. Exiting.")
				return
			}
		}
	}

}
