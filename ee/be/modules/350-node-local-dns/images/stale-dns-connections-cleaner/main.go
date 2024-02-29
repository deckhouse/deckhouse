/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/binary"
	"errors"
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
	log.Infof("Start")
	log.Infof("This is a workaround for issue https://github.com/cilium/cilium/issues/31012.")
	log.Infof("When both the cni-cilium and node-local-dns modules are enabled, and the node-local-dns pod has been restarted, stale DNS connections may occur.")
	log.Infof("This is due to the UDP socket remaining active in the application pods with the destination IP address of the old node-local-dns pod, which has already been deleted.")
	log.Infof("To prevent this problem, the following actions are taken:")
	log.Infof("- Obtain the name and PodCidr of the node where the application is running.")
	log.Infof("- Then every 30 seconds:")
	log.Infof("  - Retrieve the current IP address of the node-local-dns pod.")
	log.Infof("  - Retrieve all UDP sockets on the node and search for those with dst_port 53 and dsp_ip belonging to PodCidr, but not equal to the current IP address of the node-local-dns pod.")
	log.Infof("  - If such sockets are found, delete them.")

	// Init kubeClient
	config, _ := rest.InClusterConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to init kubeClient. Error: %v", err)
	}
	log.Infof("kubeClient successfully inited")

	// Get name of node
	currentNodeName := os.Getenv("NODE_NAME")
	if len(currentNodeName) == 0 {
		log.Fatalf("Failed to get env NODE_NAME.")
	}
	log.Infof("The current node name is %s", currentNodeName)

	// Get podCIDR of the node
	podCIDROnSameNode, err := getPodCIDR(kubeClient, currentNodeName)
	if err != nil {
		log.Fatalf("Failed to get PodCIDR of the node. Error: %v", err)
	}
	log.Infof(
		"podCIDR on node %s is %s",
		currentNodeName,
		podCIDROnSameNode.String(),
	)

	// Main loop
	for {
		time.Sleep(scanInterval)
		// Get ip of pod node-local-dns running on the node
		nldPodNameOnSameNode, nldPodIPOnSameNode, err := getNLDPodNameAndIPByNodeName(kubeClient, currentNodeName)
		if err != nil {
			log.Errorf("Failed to get IP of the nld Pod. Error: %v", err)
			continue
		}
		if nldPodIPOnSameNode == nil {
			log.Errorf("The IP address has not yet been assigned to the pod.")
			continue
		}

		// Get all UDP sockets on node
		allUDPSockets, err := netlink.SocketDiagUDP(familyIPv4)
		if err != nil {
			log.Errorf("Failed get UDP sockets. Error: %v", err)
		}

		/*
			For each socket check:
			- If DST Port is equal to nldDstPort?
			- Is DST IP belongs to podCIDR?
			- Isn't DST IP equal to nldPodIP?
			If all checks are true, then delete such socket
		*/
		for _, sock := range allUDPSockets {
			if !(sock.ID.DestinationPort == nldDstPort) {
				// this is not dns connection
				continue
			}
			if !podCIDROnSameNode.Contains(sock.ID.Destination) {
				// this connection is not to our node's PodCIDR
				continue
			}
			if sock.ID.Destination.Equal(nldPodIPOnSameNode) {
				// this connection to working node-local-dns Pod, appropriate one
				continue
			}
			// the others sockets are inappropriate, let's drop them

			log.Infof(
				"Found socket %s:%v -> %s:%v, where dst_ip is belongs to the podCIDR (%s) and dst_port is equal %v.",
				sock.ID.Source.String(),
				sock.ID.SourcePort,
				sock.ID.Destination.String(),
				sock.ID.DestinationPort,
				podCIDROnSameNode.String(),
				nldDstPort,
			)
			log.Infof(
				"Pod %s has ip %s. dst ip from socket(%s) is not equal to the ip of pod. So this socket will be destroyed.",
				nldPodNameOnSameNode,
				nldPodIPOnSameNode.String(),
				sock.ID.Destination.String(),
			)
			err := destroySocket(sock.ID)
			if err != nil {
				if errors.Is(err, unix.EOPNOTSUPP) {
					log.Fatalf("Failed to destroy the socket because this is not supported by underlying kernel. Error: %v", err)
				} else {
					log.Errorf("Failed to destroy the socket. Error: %v", err)
				}
			}
			log.Infof(
				"Socket %s:%v -> %s:%v successfully destroyed",
				sock.ID.Source.String(),
				sock.ID.SourcePort,
				sock.ID.Destination.String(),
				sock.ID.DestinationPort,
			)
		}
	}
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
			"Failed to get node. Error: %v",
			err,
		)
	}
	_, podCIDR, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to transform PodCIDR from String to net.IPNet. Error: %v",
			err,
		)
	}
	return podCIDR, nil
}

// Get current Pod Name and IP by Node name
func getNLDPodNameAndIPByNodeName(kubeClient kubernetes.Interface, nodeName string) (string, net.IP, error) {
	nldPodsOnSameNode, err := kubeClient.CoreV1().Pods(nldNS).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: nldLabelSelector,
			FieldSelector: "spec.nodeName=" + nodeName,
		},
	)
	if err != nil {
		return "", nil, fmt.Errorf(
			"Failed to list pods on same node. Error: %v",
			err,
		)
	}
	switch {
	case len(nldPodsOnSameNode.Items) == 0:
		return "", nil, fmt.Errorf(
			"There aren't agent pods on node %s",
			nodeName,
		)
	case len(nldPodsOnSameNode.Items) > 1:
		return "", nil, fmt.Errorf(
			"There are more than one running node-local-dns pods on node %s",
			nodeName,
		)
	}
	currentPod := nldPodsOnSameNode.Items[0]
	currentNLDPodIP := net.ParseIP(currentPod.Status.PodIP)
	return currentPod.Name, currentNLDPodIP, nil
}

// Destroy socket
func destroySocket(sockId netlink.SocketID) error {
	// Create a new netlink request
	s, err := nl.Subscribe(unix.NETLINK_INET_DIAG)
	if err != nil {
		return fmt.Errorf(
			"Failed create a new netlink request. Error: %v",
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
		return fmt.Errorf("Error destroying socket: %v", sockId)
	}
	return nil
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
