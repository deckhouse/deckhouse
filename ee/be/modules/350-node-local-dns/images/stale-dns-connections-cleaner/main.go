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
	"net/http"
	"os"
	"os/signal"
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
	nldNS            string = "kube-system"
	nldLabelSelector string = "app=node-local-dns"
	nldDstPort       uint16 = 53
	scanInterval            = 30 * time.Second
	listenAddress           = "127.0.0.1:9000"
	// netlink const
	familyIPv4          = syscall.AF_INET
	protoUDP            = unix.IPPROTO_UDP
	sizeofSocketID      = 0x30
	sizeofSocketRequest = sizeofSocketID + 0x8
	sockDestroy         = 21
)

type ConnectionsCleaner struct {
	kubeClient       kubernetes.Interface
	checkInterval    time.Duration
	listenAddress    string
	dstPort          uint16
	nameSpace        string
	podLabelSelector string
	nodeName         string
}

var (
	native       = nl.NativeEndian()
	networkOrder = binary.BigEndian
)

func main() {
	log.Infof("Start")
	defer log.Infof("Stop")
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
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to init kubeClient config. Error: %v", err)
	}
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

	// Create a new instance of ConnectionsCleaner
	nldCC := &ConnectionsCleaner{
		kubeClient:       kubeClient,
		checkInterval:    scanInterval,
		listenAddress:    listenAddress,
		dstPort:          nldDstPort,
		nameSpace:        nldNS,
		podLabelSelector: nldLabelSelector,
		nodeName:         currentNodeName,
	}

	log.Infof("Address: %v", nldCC.listenAddress)
	log.Infof("Checks interval: %v", nldCC.checkInterval)

	// channels to stop converge loop
	doneCh := make(chan struct{})

	httpServer := nldCC.getHTTPServer()

	rootCtx, cancel := context.WithCancel(context.Background())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		log.Infof("Signal received: %v. Exiting.\n", <-signalChan)
		cancel()
		log.Infoln("Waiting for stop reconcile loop...")
		<-doneCh

		ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer cancel()

		log.Infoln("Shutdown ...")

		err := httpServer.Shutdown(ctx)
		if err != nil {
			log.Fatalf("Error occurred while closing the server: %v\n", err)
		}
		os.Exit(0)
	}()

	// Get podCIDR of the node
	podCIDROnSameNode, err := nldCC.getPodCIDR(rootCtx)
	if err != nil {
		log.Fatalf("Failed to get PodCIDR of the node. Error: %v", err)
	}
	log.Infof(
		"podCIDR on node %s is %s",
		currentNodeName,
		podCIDROnSameNode.String(),
	)

	go nldCC.checkAndDestroyLoop(rootCtx, doneCh, podCIDROnSameNode)

	err = httpServer.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// Generate HTTPServer conf
func (cc *ConnectionsCleaner) getHTTPServer() *http.Server {
	indexPageContent := fmt.Sprintf(`<html>
             <head><title>Stale-dns-connections-cleaner</title></head>
             <body>
             <h1> Check connections every %s</h1>
             </body>
             </html>`, cc.checkInterval.String())

	router := http.NewServeMux()
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(indexPageContent))
	})

	return &http.Server{Addr: cc.listenAddress, Handler: router, ReadHeaderTimeout: cc.checkInterval}
}

// Main loop
func (cc *ConnectionsCleaner) checkAndDestroyLoop(ctx context.Context, doneCh chan<- struct{}, podCIDR *net.IPNet) {
	cc.checkAndDestroy(ctx, podCIDR)

	ticker := time.NewTicker(cc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cc.checkAndDestroy(ctx, podCIDR)
		case <-ctx.Done():
			doneCh <- struct{}{}
			return
		}
	}
}

// Check the connections and if they are stuck, remove them.
func (cc *ConnectionsCleaner) checkAndDestroy(ctx context.Context, podCIDR *net.IPNet) {
	nldPodNameOnSameNode, nldPodIPOnSameNode, err := cc.getNLDPodNameAndIPByNodeName(ctx)
	if err != nil {
		log.Errorf("Failed to get IP of the nld Pod. Error: %v", err)
		return
	}
	if nldPodIPOnSameNode == nil {
		log.Errorf("The IP address has not yet been assigned to the pod.")
		return
	}

	// Get all UDP sockets on node
	allUDPSockets, err := netlink.SocketDiagUDP(familyIPv4)
	if err != nil {
		log.Errorf("Failed get UDP sockets. Error: %v", err)
		return
	}

	/*
		For each socket check:
		- If DST Port is equal to nldDstPort?
		- Is DST IP belongs to podCIDR?
		- Isn't DST IP equal to nldPodIP?
		If all checks are true, then delete such socket
	*/
	for _, sock := range allUDPSockets {
		if !(sock.ID.DestinationPort == cc.dstPort) {
			// this is not dns connection
			continue
		}
		if !podCIDR.Contains(sock.ID.Destination) {
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
			podCIDR.String(),
			cc.dstPort,
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
			}
			log.Errorf("Failed to destroy the socket. Error: %v", err)
			continue
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

// Get podCIDR of node by Node name
func (cc *ConnectionsCleaner) getPodCIDR(ctx context.Context) (*net.IPNet, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	node, err := cc.kubeClient.CoreV1().Nodes().Get(
		cctx,
		cc.nodeName,
		metav1.GetOptions{},
	)
	cancel()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get node. Error: %v",
			err,
		)
	}
	_, podCIDR, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to transform PodCIDR from String to net.IPNet. Error: %v",
			err,
		)
	}
	return podCIDR, nil
}

// Get current Pod Name and IP by Node name
func (cc *ConnectionsCleaner) getNLDPodNameAndIPByNodeName(ctx context.Context) (string, net.IP, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	nldPodsOnSameNode, err := cc.kubeClient.CoreV1().Pods(cc.nameSpace).List(
		cctx,
		metav1.ListOptions{
			LabelSelector: cc.podLabelSelector,
			FieldSelector: "spec.nodeName=" + cc.nodeName,
		},
	)
	if err != nil {
		return "", nil, fmt.Errorf(
			"failed to list pods on same node. Error: %v",
			err,
		)
	}
	switch {
	case len(nldPodsOnSameNode.Items) == 0:
		return "", nil, fmt.Errorf(
			"there aren't agent pods on node %s",
			cc.nodeName,
		)
	case len(nldPodsOnSameNode.Items) > 1:
		return "", nil, fmt.Errorf(
			"there are more than one running node-local-dns pods on node %s",
			cc.nodeName,
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
			"failed create a new netlink request. Error: %v",
			err,
		)
	}
	defer s.Close()

	// Construct the request
	req := nl.NewNetlinkRequest(sockDestroy, unix.NLM_F_REQUEST)
	req.AddData(&socketRequest{
		Family:   familyIPv4,
		Protocol: protoUDP,
		States:   uint32(0xfff),
		ID:       sockId,
	})

	// Do the query
	err = s.Send(req)
	if err != nil {
		return fmt.Errorf("error destroying socket: %v", sockId)
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
