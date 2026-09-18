// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// validateClusterNetworking rejects the address-space mistakes that can be decided from the
// documents alone.
//
// They used to be preflight checks, which meant they did not run for `dhctl config`, did not run
// for converge, were not applied by Commander (which never called the preflight option
// validator), and were turned off wholesale by --preflight-skip-all-checks — a flag operators
// reach for to get past an unrelated network problem. None of these is a transient condition
// worth a retry or a skip: a pod subnet that overlaps the service subnet is wrong on every run,
// and these parameters cannot be changed after the cluster is created.
//
// Preflight keeps what needs the network, SSH, or the state of a node or a cloud.
func validateClusterNetworking(ctx context.Context, m *MetaConfig) error {
	if len(m.ClusterConfig) == 0 {
		return nil
	}

	podCIDR, err := clusterCIDRField(m, "podSubnetCIDR")
	if err != nil {
		return err
	}
	serviceCIDR, err := clusterCIDRField(m, "serviceSubnetCIDR")
	if err != nil {
		return err
	}
	if podCIDR == nil || serviceCIDR == nil {
		// Both are required by the schema; a document that reached here without them has already
		// been refused, or is one of the partial configurations converge assembles.
		return nil
	}

	if cidrsOverlap(podCIDR.network, serviceCIDR.network) {
		return fmt.Errorf(
			"%s %s overlaps %s %s. "+
				"Pod and service addresses are routed differently and cannot share a range; "+
				"use disjoint ranges (for example 10.111.0.0/16 and 10.222.0.0/16). "+
				"Neither can be changed after the cluster is created",
			podCIDR.field, podCIDR.value, serviceCIDR.field, serviceCIDR.value)
	}

	if err := validateInternalNetworkCIDRs(m, podCIDR, serviceCIDR); err != nil {
		return err
	}

	if err := validateServiceSubnetSize(serviceCIDR); err != nil {
		return err
	}

	if err := validatePodSubnetNodeCIDRPrefix(ctx, m, podCIDR); err != nil {
		return err
	}

	return validatePublicDomainTemplate(m)
}

// clusterCIDR is a parsed field, kept with the text the operator wrote so messages can quote it.
type clusterCIDR struct {
	field   string
	value   string
	network *net.IPNet
}

// networkFieldLabel names a parameter where the operator will find it. #22688 moved these three
// to ModuleConfig control-plane-manager and left the ClusterConfiguration fields as a deprecated
// fallback, so the document to correct is whichever one the value actually came from — quoting
// the wrong one sends the reader to edit a field that is not there.
func (m *MetaConfig) networkFieldLabel(name string) string {
	for _, p := range m.networkParams() {
		if p.name == name && p.mc != "" {
			return "ModuleConfig control-plane-manager spec.settings.network." + name
		}
	}
	return "ClusterConfiguration." + name
}

// clusterCIDRField reads one CIDR through Network(), which resolves ModuleConfig over the
// deprecated ClusterConfiguration field. Reading ClusterConfig directly is what this used to do,
// and after #22688 that is empty for every migrated cluster — so every comparison below found
// nothing to compare and the whole of validateClusterNetworking became a no-op without saying so.
func clusterCIDRField(m *MetaConfig, field string) (*clusterCIDR, error) {
	value := ""
	switch field {
	case "podSubnetCIDR":
		value = m.Network().PodSubnetCIDR
	case "serviceSubnetCIDR":
		value = m.Network().ServiceSubnetCIDR
	default:
		return nil, fmt.Errorf("unknown cluster CIDR field %q", field)
	}

	if value == "" {
		return nil, nil
	}

	label := m.networkFieldLabel(field)

	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return nil, fmt.Errorf(
			"%s %q is not an IPv4 CIDR: write an address and a prefix length, for example 10.111.0.0/16",
			label, value)
	}
	return &clusterCIDR{field: label, value: value, network: network}, nil
}

func cidrsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

// validateInternalNetworkCIDRs compares the cluster subnets with the networks a static cluster
// says its nodes talk to each other over.
func validateInternalNetworkCIDRs(m *MetaConfig, cidrs ...*clusterCIDR) error {
	raw, ok := m.StaticClusterConfig["internalNetworkCIDRs"]
	if !ok || len(raw) == 0 {
		return nil
	}

	var internal []string
	if err := json.Unmarshal(raw, &internal); err != nil {
		return fmt.Errorf("StaticClusterConfiguration.internalNetworkCIDRs is not a list of CIDRs")
	}

	for _, entry := range internal {
		_, internalNet, err := net.ParseCIDR(entry)
		if err != nil {
			return fmt.Errorf(
				"StaticClusterConfiguration.internalNetworkCIDRs contains %q, which is not an IPv4 CIDR", entry)
		}

		for _, cidr := range cidrs {
			if cidrsOverlap(cidr.network, internalNet) {
				return fmt.Errorf(
					"%s %s overlaps StaticClusterConfiguration.internalNetworkCIDRs entry %s. "+
						"The cluster would route its own traffic into the network the nodes reach each other over; "+
						"use a range the nodes do not use",
					cidr.field, cidr.value, entry)
			}
		}
	}
	return nil
}

// serviceSubnetMinimumPrefix is the narrowest service subnet that still holds the cluster DNS
// address. getDNSAddress takes the eleventh address of the subnet, so anything narrower than a
// /28 leaves the cluster with no DNS address at all — and it returns "" for it silently.
const serviceSubnetMinimumPrefix = 28

func validateServiceSubnetSize(serviceCIDR *clusterCIDR) error {
	ones, bits := serviceCIDR.network.Mask.Size()
	if bits != 32 {
		return nil // IPv6 is not something dhctl creates clusters with.
	}
	if ones <= serviceSubnetMinimumPrefix {
		return nil
	}

	return fmt.Errorf(
		"%s %s is too small: the cluster DNS address is the eleventh address of "+
			"the service subnet, so the range must be /%d or wider (a /16 is the usual choice)",
		serviceCIDR.field, serviceCIDR.value, serviceSubnetMinimumPrefix)
}

// podSubnetNodeCIDRPrefixMax is the narrowest slice a node can be given and still run pods: a /28
// leaves 14 addresses.
const podSubnetNodeCIDRPrefixMax = 28

// validatePodSubnetNodeCIDRPrefix checks the per-node slice of the pod network against the pod
// network itself, and — as a warning — against the number of nodes the configuration asks for.
//
// The structural part is an error: a prefix that is not larger than the pod subnet's own prefix
// gives every node the whole network, and one narrower than /28 gives it almost no addresses.
// The capacity part is a warning: an existing cluster may legitimately have grown past what its
// configuration would allow today, and converge re-runs this.
func validatePodSubnetNodeCIDRPrefix(ctx context.Context, m *MetaConfig, podCIDR *clusterCIDR) error {
	// Network() resolves this one the same way as the CIDRs, and falls back to
	// DefaultPodSubnetNodeCIDRPrefix when neither document sets it — which is the value the
	// cluster will actually be built with, so it is the value worth validating.
	value := m.Network().PodSubnetNodeCIDRPrefix
	if value == "" {
		return nil
	}

	label := m.networkFieldLabel("podSubnetNodeCIDRPrefix")

	nodePrefix, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf(
			"%s %q is not a number: it is the prefix length of the slice "+
				"each node receives, for example \"24\"", label, value)
	}

	podPrefix, _ := podCIDR.network.Mask.Size()
	if nodePrefix <= podPrefix {
		return fmt.Errorf(
			"%s %q must be larger than the prefix of podSubnetCIDR %s: "+
				"every node receives a /%d slice of it, and /%d is the whole network. "+
				"Use a value between %d and %d (the default is \"24\")",
			label, value, podCIDR.value, nodePrefix, nodePrefix, podPrefix+1, podSubnetNodeCIDRPrefixMax)
	}
	if nodePrefix > podSubnetNodeCIDRPrefixMax {
		return fmt.Errorf(
			"%s %q leaves a node too few addresses for its pods: "+
				"use /%d or wider (the default is \"24\")", label, value, podSubnetNodeCIDRPrefixMax)
	}

	capacity := 1 << (nodePrefix - podPrefix)
	if requested := m.requestedNodeCount(); requested > 0 && capacity < requested {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"podSubnetCIDR %s with podSubnetNodeCIDRPrefix %q allows %d nodes, "+
				"and the configuration asks for %d. Widen podSubnetCIDR or raise podSubnetNodeCIDRPrefix; "+
				"neither can be changed after the cluster is created",
			podCIDR.value, value, capacity, requested))
	}

	return nil
}

// requestedNodeCount is how many nodes the documents name outright. CloudEphemeral groups scale
// on their own and are not counted, so this is a floor rather than a total — which is why falling
// short of it is worth a warning and not an error.
func (m *MetaConfig) requestedNodeCount() int {
	count := m.MasterNodeGroupSpec.Replicas
	for _, group := range m.TerraNodeGroupSpecs {
		count += group.Replicas
	}
	return count
}

// validatePublicDomainTemplate keeps Ingress host names out of the in-cluster DNS zone.
//
// The comparison is on label boundaries and case-insensitive. As a preflight check it was
// strings.Contains, which rejected "%s.mycluster.localnet.io" under clusterDomain "cluster.local"
// and the documented "%s.k8s.internal.example.com" under "k8s.internal", while letting
// "Cluster.Local" through.
func validatePublicDomainTemplate(m *MetaConfig) error {
	if m.ClusterDomain == "" {
		return nil
	}

	mc := m.FindModuleConfig("global")
	if mc == nil {
		return nil
	}

	modules, ok := mc.Spec.Settings["modules"].(map[string]any)
	if !ok {
		return nil
	}
	template, ok := modules["publicDomainTemplate"].(string)
	if !ok || strings.TrimSpace(template) == "" {
		return nil
	}

	if !domainIsInside(template, m.ClusterDomain) {
		return nil
	}

	return fmt.Errorf(
		"publicDomainTemplate %q is inside clusterDomain %q: Ingress host names would collide with in-cluster DNS "+
			"names. Set spec.settings.modules.publicDomainTemplate in the \"global\" ModuleConfig to a domain outside "+
			"%q (for example \"%%s.example.com\"), or change ClusterConfiguration.clusterDomain",
		template, m.ClusterDomain, m.ClusterDomain)
}

func domainIsInside(template, domain string) bool {
	template = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(template), "."))
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if template == "" || domain == "" {
		return false
	}
	return template == domain || strings.HasSuffix(template, "."+domain)
}
