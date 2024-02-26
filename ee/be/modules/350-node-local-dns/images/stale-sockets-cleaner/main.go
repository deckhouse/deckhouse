/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	nldNS               string = "kube-system"
	nldLabelSelector    string = "app=node-local-dns"
	nldDstPort          uint16 = 53
	scanInterval               = 30 * time.Second
	sizeofSocketID             = 0x30
	sizeofSocketRequest        = sizeofSocketID + 0x8
	SOCK_DESTROY               = 21
	familyIPv4                 = syscall.AF_INET
	protoUDP                   = unix.IPPROTO_UDP
)

var (
	native       = nl.NativeEndian()
	networkOrder = binary.BigEndian
)

func main() {
	log.Infof("[StaleSockCleaner] Start")

	// Get name of node
	currentNodeName := os.Getenv("NODE_NAME")
	if len(currentNodeName) == 0 {
		log.Fatalf("[StaleSockCleaner] Failed to get env NODE_NAME.")
	}

	// Init kubeClient
	config, _ := rest.InClusterConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[StaleSockCleaner] Failed to init kubeClient. Error: %v", err)
	}

	// Get podCIDR of the node
	podCIDROnSameNode, err := getPodCIDR(kubeClient, currentNodeName)
	if err != nil {
		log.Fatalf("[StaleSockCleaner] Failed to get PodCIDR of the node. Error: %v", err)
	}

	for {
		// Get ip of pod node-local-dns running on the node
		nldPodIPOnSameNode, err := getNLDPodIP(kubeClient, currentNodeName)
		if err != nil {
			log.Fatalf("[StaleSockCleaner] Failed to get IP of the nld Pod. Error: %v", err)
		}

		// Get all UDP sockets on node
		allUDPSockets, err := netlink.SocketDiagUDP(familyIPv4)
		if err != nil {
			log.Fatalf("[StaleSockCleaner] Failed get UPD sockets. Error: %v", err)
		}

		// For each socket do something
		for _, sock := range allUDPSockets {
			// Print socket
			log.Infof("[StaleSockCleaner] Socket: %s:%v -> %s:%v",
				sock.ID.Source.String(),
				sock.ID.SourcePort,
				sock.ID.Destination.String(),
				sock.ID.DestinationPort,
			)
			// Check: is the DestinationPort equil nldDstPort and the Destination IP contained in the podCIDR?
			if sock.ID.DestinationPort == nldDstPort &&
				podCIDROnSameNode.Contains(sock.ID.Destination) {
				log.Infof(
					"[StaleSockCleaner] DestinationPort of this socket equil nldDstPort and Destination IP is contained in the podCIDR",
				)
				if !sock.ID.Destination.Equal(nldPodIPOnSameNode) {
					log.Infof(
						"[StaleSockCleaner] Destination IP of this socket is not equil IP of nld Pod",
					)
					err := destroySocket(sock.ID)
					if err != nil {
						log.Fatalf("[StaleSockCleaner] Failed destroy socket. Error: %v", err)
					}
				}
			}
		}
		time.Sleep(scanInterval)
	}
	// log.Infof("[StaleSockCleaner] End")
}

// Get podCIDR of node by Node name
func getPodCIDR(kubeClient kubernetes.Interface, nodeName string) (*net.IPNet, error) {
	node, err := kubeClient.CoreV1().Nodes().Get(
		context.TODO(),
		nodeName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"[StaleSockCleaner] Failed to get node. Error: %v",
			err,
		)
	}
	_, podCIDR, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return nil, fmt.Errorf(
			"[StaleSockCleaner] Failed to transform PodCIDR from String to net.IPNet. Error: %v",
			err,
		)
	}
	return podCIDR, nil
}

// Get current Pod IP by Node name
func getNLDPodIP(kubeClient kubernetes.Interface, nodeName string) (net.IP, error) {
	nldPodsOnSameNode, err := kubeClient.CoreV1().Pods(nldNS).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: nldLabelSelector,
			FieldSelector: "spec.nodeName=" + nodeName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"[StaleSockCleaner] Failed to list pods on same node. Error: %v",
			err,
		)
	}
	log.Infof(
		"[StaleSockCleaner] Count of nld running on node %s is %v",
		nodeName,
		len(nldPodsOnSameNode.Items),
	)
	switch {
	case len(nldPodsOnSameNode.Items) == 0:
		return nil, fmt.Errorf(
			"[StaleSockCleaner] There aren't agent pods on node %s",
			nodeName,
		)
	case len(nldPodsOnSameNode.Items) > 1:
		return nil, fmt.Errorf(
			"[StaleSockCleaner] There are more than one running agent pods on node %s",
			nodeName,
		)
	}
	currentPod := nldPodsOnSameNode.Items[0]
	log.Infof(
		"[StaleSockCleaner] Name of nls pod which running on the same node is %s",
		currentPod.Name,
	)
	log.Infof(
		"[StaleSockCleaner] IP of pod %s which running on the same node is %s",
		currentPod.Name,
		currentPod.Status.PodIP,
	)
	currentNLDPodIP := net.ParseIP(currentPod.Status.PodIP)
	return currentNLDPodIP, nil
}

// Destroy socket
func destroySocket(sockId netlink.SocketID) error {
	// Create a new netlink request
	s, err := nl.Subscribe(unix.NETLINK_INET_DIAG)
	if err != nil {
		return fmt.Errorf(
			"[StaleSockCleaner] Failed create a new netlink request. Error: %v",
			err,
		)
	}
	defer s.Close()

	// Construct the request
	req := nl.NewNetlinkRequest(SOCK_DESTROY, unix.NLM_F_REQUEST)
	req.AddData(&socketRequest{
		Family:   familyIPv4,
		Protocol: protoUDP,
		States:   uint32(0xfff),
		ID:       sockId,
	})

	// Do the query
	err = s.Send(req)
	if err != nil {
		fmt.Printf("[StaleSockCleaner] error in destroying socket: %v", sockId)
	}
	return err
}

// Below handlers are adapted from netlink/socket_linux.go
type writeBuffer struct {
	Bytes []byte
	pos   int
}

func (b *writeBuffer) write(c byte) {
	b.Bytes[b.pos] = c
	b.pos++
}

func (b *writeBuffer) next(n int) []byte {
	s := b.Bytes[b.pos : b.pos+n]
	b.pos += n
	return s
}

type socketRequest struct {
	Family   uint8
	Protocol uint8
	Ext      uint8
	pad      uint8
	States   uint32
	ID       netlink.SocketID
}

func (r *socketRequest) Serialize() []byte {
	b := writeBuffer{Bytes: make([]byte, sizeofSocketRequest)}
	b.write(r.Family)
	b.write(r.Protocol)
	b.write(r.Ext)
	b.write(r.pad)
	native.PutUint32(b.next(4), r.States)
	networkOrder.PutUint16(b.next(2), r.ID.SourcePort)
	networkOrder.PutUint16(b.next(2), r.ID.DestinationPort)
	if r.Family == unix.AF_INET6 {
		copy(b.next(16), r.ID.Source)
		copy(b.next(16), r.ID.Destination)
	} else {
		copy(b.next(4), r.ID.Source.To4())
		b.next(12)
		copy(b.next(4), r.ID.Destination.To4())
		b.next(12)
	}
	native.PutUint32(b.next(4), r.ID.Interface)
	native.PutUint32(b.next(4), r.ID.Cookie[0])
	native.PutUint32(b.next(4), r.ID.Cookie[1])
	return b.Bytes
}

func (r *socketRequest) Len() int { return sizeofSocketRequest }

// End of handlers
