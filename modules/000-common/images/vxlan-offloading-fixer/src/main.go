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
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var (
	pathToEthtool = "/ethtool"
)

func main() {
	// logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// slog.SetDefault(logger)

	nodeIP := os.Getenv("NODE_IP")
	if nodeIP == "" {
		log.Fatal("NODE_IP environment variable is not set")
	}

	driverRegexp := regexp.MustCompile(`^driver:\s*(\S+)`)
	udpSegmentationRegexp := regexp.MustCompile(`tx-udp_tnl-segmentation:\s*(\S+)`)
	udpCsumSegmentationRegexp := regexp.MustCompile(`tx-udp_tnl-csum-segmentation:\s*(\S+)`)

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatalf("could not get network interfaces: %+v\n", err.Error())
	}

	log.Println("switching off network interfaces vxlan offloading...")
	for _, iface := range ifaces {
		if iface.Name == "lo" {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			log.Printf("could not get addresses for interface: %+v\n", err.Error())
			continue
		}
		found := false
		for _, address := range addresses {
			log.Printf("checking address %s for interface %s\n", address.String(), iface.Name)
			addressParts := strings.Split(address.String(), "/")
			if addressParts[0] == nodeIP {
				found = true
				log.Printf("found NODE_IP address %s for interface %s\n", nodeIP, iface.Name)
				break
			}
		}
		if !found {
			continue
		}

		cmd := exec.Command(pathToEthtool, "-i", iface.Name)
		ifaceInfo, err := cmd.Output()
		if err != nil {
			log.Printf("could not get driver info for network interface %s: %+v\n", iface.Name, err.Error())
			continue
		}

		drvMatches := driverRegexp.FindSubmatch(ifaceInfo)
		if len(drvMatches) == 0 || string(drvMatches[1]) != "vmxnet3" {
			continue // we are only interested in network interfaces with the driver vmxnet3
		}

		cmd = exec.Command(pathToEthtool, "-k", iface.Name)
		offloadInfo, err := cmd.Output()
		if err != nil {
			log.Printf("could not get features info for network interface %s: %+v\n", iface.Name, err.Error())
			continue
		}

		// Fix UDP segmentation
		udpSegmentationMatches := udpSegmentationRegexp.FindSubmatch(offloadInfo)
		if len(udpSegmentationMatches) > 0 && string(udpSegmentationMatches[1]) == "on" {
			log.Println("switching off udp segmentation for network interfaces:", iface.Name)
			cmd = exec.Command(pathToEthtool, "-K", iface.Name, "tx-udp_tnl-segmentation", "off")
			if err := cmd.Run(); err != nil {
				log.Printf("could not switch off udp segmentation for network interface %s: %+v\n", iface.Name, err.Error())
			}
		}

		// Fix UDP checksum segmentation
		udpChsumSegmentMatches := udpCsumSegmentationRegexp.FindSubmatch(offloadInfo)
		if len(udpChsumSegmentMatches) > 0 && string(udpChsumSegmentMatches[1]) == "on" {
			log.Println("fix udp checksum segmentation for network interfaces:", iface.Name)
			cmd = exec.Command(pathToEthtool, "-K", iface.Name, "tx-udp_tnl-csum-segmentation", "off")
			if err := cmd.Run(); err != nil {
				log.Printf("could not switch off udp checksum segmentation for network interface %s: %+v\n", iface.Name, err.Error())
			}
		}
	}
	log.Println("correction is complete")
}
