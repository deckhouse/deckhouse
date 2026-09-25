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
		return configurationFailure(
			fmt.Sprintf("%s against %s", podCIDR.field, serviceCIDR.field),
			fmt.Sprintf("%s overlaps %s", podCIDR.value, serviceCIDR.value),
			"two ranges that do not overlap",
			fmt.Sprintf("set %s to a range outside %s, for example 10.222.0.0/16",
				shortFieldName(serviceCIDR.field), shortFieldName(podCIDR.field)))
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
		return nil, configurationFailure(
			label,
			fmt.Sprintf("%q", value),
			"an IPv4 CIDR",
			fmt.Sprintf("set %s to an address with a prefix length, for example 10.111.0.0/16", shortFieldName(label)))
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
		return configurationFailure(
			"StaticClusterConfiguration.internalNetworkCIDRs",
			"not a list",
			"a list of IPv4 CIDRs",
			`write internalNetworkCIDRs as a list, for example ["192.168.0.0/24"]`)
	}

	for _, entry := range internal {
		_, internalNet, err := net.ParseCIDR(entry)
		if err != nil {
			return configurationFailure(
				"StaticClusterConfiguration.internalNetworkCIDRs",
				fmt.Sprintf("entry %q", entry),
				"an IPv4 CIDR",
				"set the entry to an address with a prefix length, for example 192.168.0.0/24")
		}

		for _, cidr := range cidrs {
			if cidrsOverlap(cidr.network, internalNet) {
				return configurationFailure(
					fmt.Sprintf("%s against StaticClusterConfiguration.internalNetworkCIDRs", cidr.field),
					fmt.Sprintf("%s overlaps entry %s", cidr.value, entry),
					"two ranges that do not overlap",
					fmt.Sprintf("set %s to a range outside internalNetworkCIDRs, for example 10.222.0.0/16",
						shortFieldName(cidr.field)))
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

	return configurationFailure(
		serviceCIDR.field,
		fmt.Sprintf("%s, narrower than /%d", serviceCIDR.value, serviceSubnetMinimumPrefix),
		fmt.Sprintf("/%d or wider. The cluster DNS address is the eleventh address of the subnet.",
			serviceSubnetMinimumPrefix),
		fmt.Sprintf("set %s to a wider range, for example 10.222.0.0/16", shortFieldName(serviceCIDR.field)))
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
		return configurationFailure(
			label,
			fmt.Sprintf("%q", value),
			"a prefix length written as a number",
			fmt.Sprintf("set %s to \"24\"", shortFieldName(label)))
	}

	podPrefix, _ := podCIDR.network.Mask.Size()
	if nodePrefix <= podPrefix {
		return configurationFailure(
			fmt.Sprintf("%s against %s", label, podCIDR.field),
			fmt.Sprintf("%q, and podSubnetCIDR is %s", value, podCIDR.value),
			fmt.Sprintf("a value between %d and %d", podPrefix+1, podSubnetNodeCIDRPrefixMax),
			fmt.Sprintf("set %s to \"24\"", shortFieldName(label)))
	}
	if nodePrefix > podSubnetNodeCIDRPrefixMax {
		return configurationFailure(
			label,
			fmt.Sprintf("%q", value),
			fmt.Sprintf("%d or lower", podSubnetNodeCIDRPrefixMax),
			fmt.Sprintf("set %s to \"24\"", shortFieldName(label)))
	}

	capacity := 1 << (nodePrefix - podPrefix)
	if requested := m.requestedNodeCount(); requested > 0 && capacity < requested {
		dhlog.FromContext(ctx).WarnContext(ctx, configurationFailure(
			fmt.Sprintf("%s against the replicas the configuration asks for", podCIDR.field),
			fmt.Sprintf("%s with podSubnetNodeCIDRPrefix %q allows %d nodes, the configuration asks for %d",
				podCIDR.value, value, capacity, requested),
			"room for every node the configuration asks for. Neither field can be changed after the cluster is created.",
			"widen podSubnetCIDR, or raise podSubnetNodeCIDRPrefix").Error())
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

	domainLabel := m.networkFieldLabel("clusterDomain")

	return configurationFailure(
		"publicDomainTemplate in the \"global\" ModuleConfig against "+domainLabel,
		fmt.Sprintf("%q is inside %q", template, m.ClusterDomain),
		"a template outside clusterDomain",
		"set spec.settings.modules.publicDomainTemplate to \"%s.example.com\", "+
			"or change "+domainLabel)
}

func domainIsInside(template, domain string) bool {
	template = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(template), "."))
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if template == "" || domain == "" {
		return false
	}
	return template == domain || strings.HasSuffix(template, "."+domain)
}

// shortFieldName is the bare field, for the sentence that tells the reader what to set. The
// qualified form names the document and belongs in "checked"; repeating it inside the fix makes
// the instruction longer than the thing it instructs.
func shortFieldName(qualified string) string {
	if i := strings.LastIndex(qualified, "."); i >= 0 {
		return qualified[i+1:]
	}
	return qualified
}
