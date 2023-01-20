/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"fmt"
	"net"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/490-virtualization/hooks/internal/v1alpha1"
)

const (
	ipLeasesSnapshot = "ipamLeases"
	ipClaimsSnapshot = "ipamClaims"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/virtualization/ipam-handler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       ipLeasesSnapshot,
			ApiVersion: gv,
			Kind:       "VirtualMachineIPAddressLease",
			FilterFunc: applyVirtualMachineIPAddressLeaseFilter,
		},
		{
			Name:       ipClaimsSnapshot,
			ApiVersion: gv,
			Kind:       "VirtualMachineIPAddressClaim",
			FilterFunc: applyVirtualMachineIPAddressClaimFilter,
		},
	},
}, doIPAM)

type VirtualMachineIPAddressLeaseSnapshot struct {
	Name           string
	ClaimName      string
	ClaimNamespace string
	Address        string
	Phase          string
}

type VirtualMachineIPAddressClaimSnapshot struct {
	Name                string
	Namespace           string
	LeaseName           string
	Address             string
	Phase               string
	VMName              string
	Static              *bool
	OwnerReferenceIsSet bool
}

func applyVirtualMachineIPAddressLeaseFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	lease := &v1alpha1.VirtualMachineIPAddressLease{}
	leaseSnap := new(VirtualMachineIPAddressLeaseSnapshot)
	err := sdk.FromUnstructured(obj, lease)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to VirtualMachineIPAddressLease: %v", err)
	}

	leaseSnap.Address = nameToIP(lease.Name)
	if leaseSnap.Address == "" {
		return nil, fmt.Errorf("cannot convert VirtualMachineIPAddressLease name to IP address: %s", lease.Name)
	}

	leaseSnap.Name = lease.Name
	leaseSnap.Phase = lease.Status.Phase
	if lease.Spec.ClaimRef != nil {
		leaseSnap.ClaimName = lease.Spec.ClaimRef.Name
		leaseSnap.ClaimNamespace = lease.Spec.ClaimRef.Namespace
	}

	return leaseSnap, nil
}

func applyVirtualMachineIPAddressClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	claim := &v1alpha1.VirtualMachineIPAddressClaim{}
	err := sdk.FromUnstructured(obj, claim)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to VirtualMachineIPAddressClaim: %v", err)
	}

	return &VirtualMachineIPAddressClaimSnapshot{
		Name:                claim.Name,
		Namespace:           claim.Namespace,
		LeaseName:           claim.Spec.LeaseName,
		Static:              claim.Spec.Static,
		Address:             claim.Spec.Address,
		Phase:               claim.Status.Phase,
		VMName:              claim.Status.VMName,
		OwnerReferenceIsSet: len(claim.OwnerReferences) != 0,
	}, nil
}

type IPAM struct {
	allocatedIPs map[string]struct{}
	parsedCIDRs  []*net.IPNet
	input        *go_hook.HookInput
}

// doIPAM
//
// synopsis:
//
//	  This hook performs IPAM (IP Address Management) for VirtualMachines.
//	  It takes free IP addresses from `virtualization.vmCIDRs` and assigning them to Virtual Machines.
//	  Every VM required static IP address obtans static IP lease with specific IP address,
//	  otherwise the next free IP address assigned to the VM.
//	  Additionally this hook performs the check to make sure that requested IP address is not
//		 assigned to other Virtual Machine.
func doIPAM(input *go_hook.HookInput) error {
	ipam := IPAM{
		input:        input,
		allocatedIPs: make(map[string]struct{}),
	}

	leaseSnap := input.Snapshots[ipLeasesSnapshot]
	claimSnap := input.Snapshots[ipClaimsSnapshot]

	if len(claimSnap) == 0 && len(leaseSnap) == 0 {
		input.LogEntry.Warnln("VirtualMachineIPAddressLease and VirtualMachineIPAddressClaim not found. Skip")
		return nil
	}

	for _, sRaw := range leaseSnap {
		lease := sRaw.(*VirtualMachineIPAddressLeaseSnapshot)
		if err := ipam.loadAndProcessIPAddressLease(lease); err != nil {
			return err
		}
	}

	if err := ipam.loadCIDRsFromConfig(); err != nil {
		return err
	}

	for _, sRaw := range claimSnap {
		claim := sRaw.(*VirtualMachineIPAddressClaimSnapshot)
		if err := ipam.processIPAddressClaim(claim); err != nil {
			return err
		}
	}

	return nil
}

func (ipam *IPAM) allocateIP(address string) error {
	ip := net.ParseIP(address)
	if _, ok := ipam.allocatedIPs[ip.String()]; !ok {
		for _, cidr := range ipam.parsedCIDRs {
			if cidr.Contains(ip) {
				ipam.allocatedIPs[ip.String()] = struct{}{}
				return nil
			}
		}
		return fmt.Errorf("OutOfRange")
	}
	return fmt.Errorf("Conflict")
}

func (ipam *IPAM) allocateNewIP() (string, error) {
	for _, cidr := range ipam.parsedCIDRs {
		ip := cidr.IP
		for ip := ip.Mask(cidr.Mask); cidr.Contains(ip); inc(ip) {
			if _, ok := ipam.allocatedIPs[ip.String()]; !ok {
				ipam.allocatedIPs[ip.String()] = struct{}{}
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("NoRemainingIPs")
}

// http://play.golang.org/p/m8TNTtygK0
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (ipam *IPAM) createLeaseForClaim(claim *VirtualMachineIPAddressClaimSnapshot) *VirtualMachineIPAddressLeaseSnapshot {
	var ip string
	var err error

	if claim.Address != "" {
		ip = claim.Address
		err = ipam.allocateIP(claim.Address)
	} else {
		ip, err = ipam.allocateNewIP()
	}

	if err != nil {
		switch err.Error() {
		case "OutOfRange":
			ipam.input.LogEntry.Warnf("error allocating ip %s, not in CIDRs %s", ip, ipam.parsedCIDRs)
		case "Conflict":
			ipam.input.LogEntry.Warnf("error allocating ip %s, already allocated", ip)
		case "NoRemainingIPs":
			ipam.input.LogEntry.Warnf("error allocating ip, no remaining IPs found")
		}

		if claim.LeaseName != "" {
			patch := map[string]interface{}{"spec": map[string]interface{}{"leaseName": nil}}
			ipam.input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", claim.Namespace, claim.Name)
		}
		patch := map[string]interface{}{"status": map[string]interface{}{"phase": err.Error()}}
		ipam.input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", claim.Namespace, claim.Name, object_patch.WithSubresource("/status"))
		return nil
	}

	leaseName := ipToName(ip)

	lease := &v1alpha1.VirtualMachineIPAddressLease{}
	setLeaseFields(lease, leaseName, claim)
	ipam.input.PatchCollector.Create(lease)

	return &VirtualMachineIPAddressLeaseSnapshot{
		Name:           leaseName,
		ClaimName:      claim.Name,
		ClaimNamespace: claim.Namespace,
		Address:        ip,
	}
}

func setLeaseFields(leaseObj *v1alpha1.VirtualMachineIPAddressLease, leaseName string, claim *VirtualMachineIPAddressClaimSnapshot) {
	leaseObj.TypeMeta = metav1.TypeMeta{
		Kind:       "VirtualMachineIPAddressLease",
		APIVersion: gv,
	}
	leaseObj.SetName(leaseName)
	leaseObj.Spec.ClaimRef = &v1alpha1.VirtualMachineIPAddressLeaseClaimRef{
		Name:      claim.Name,
		Namespace: claim.Namespace,
	}
}

func setClaimFields(claimObj *v1alpha1.VirtualMachineIPAddressClaim, lease *VirtualMachineIPAddressLeaseSnapshot) {
	claimObj.TypeMeta = metav1.TypeMeta{
		Kind:       "VirtualMachineIPAddressClaim",
		APIVersion: gv,
	}
	static := claimObj.Spec.Static == nil || *claimObj.Spec.Static // defaults nil to true
	claimObj.Spec.Static = pointer.Bool(static)
	claimObj.Spec.LeaseName = lease.Name
	claimObj.Spec.Address = lease.Address
}

func (ipam *IPAM) patchLeaseForClaim(lease *VirtualMachineIPAddressLeaseSnapshot, claim *VirtualMachineIPAddressClaimSnapshot) {
	apply := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		leaseObj := &v1alpha1.VirtualMachineIPAddressLease{}
		err := sdk.FromUnstructured(u, leaseObj)
		if err != nil {
			return nil, err
		}
		setLeaseFields(leaseObj, lease.Name, claim)
		return sdk.ToUnstructured(&leaseObj)
	}
	ipam.input.PatchCollector.Filter(apply, gv, "VirtualMachineIPAddressLease", "", lease.Name)
}

func (ipam *IPAM) patchLeaseStatusForClaim(lease *VirtualMachineIPAddressLeaseSnapshot) {
	if lease.Phase != "Bound" {
		patch := map[string]interface{}{"status": map[string]interface{}{"phase": "Bound"}}
		ipam.input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressLease", "", lease.Name, object_patch.WithSubresource("/status"))
	}
}

func (ipam *IPAM) patchClaimForLease(lease *VirtualMachineIPAddressLeaseSnapshot, claim *VirtualMachineIPAddressClaimSnapshot) {
	apply := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		claimObj := &v1alpha1.VirtualMachineIPAddressClaim{}
		err := sdk.FromUnstructured(u, claimObj)
		if err != nil {
			return nil, err
		}
		setClaimFields(claimObj, lease)
		return sdk.ToUnstructured(&claimObj)
	}
	ipam.input.PatchCollector.Filter(apply, gv, "VirtualMachineIPAddressClaim", claim.Namespace, claim.Name)

	if claim.Phase != "Bound" {
		patch := map[string]interface{}{"status": map[string]interface{}{"phase": "Bound"}}
		ipam.input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", claim.Namespace, claim.Name, object_patch.WithSubresource("/status"))
	}
}

func (ipam *IPAM) claimIsValidForLease(lease *VirtualMachineIPAddressLeaseSnapshot, claim *VirtualMachineIPAddressClaimSnapshot) bool {
	// If lease ClaimRef not matches our claim
	if lease.ClaimName == claim.Name && lease.ClaimNamespace == claim.Namespace {
		return true
	}
	// If old claim is removed, we still can consider it as correct
	claimSnap := ipam.input.Snapshots[ipClaimsSnapshot]
	return getIPClaim(&claimSnap, lease.ClaimNamespace, lease.ClaimName) == nil
}

func (ipam *IPAM) handleConflictedClaim(claim *VirtualMachineIPAddressClaimSnapshot) {
	// Wrong lease specified, remove leaseName field
	if claim.LeaseName != "" {
		patch := map[string]interface{}{"spec": map[string]interface{}{"leaseName": nil}}
		ipam.input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", claim.Namespace, claim.Name)
	}
	if claim.Phase != "Conflict" {
		patch := map[string]interface{}{"status": map[string]interface{}{"phase": "Conflict"}}
		ipam.input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", claim.Namespace, claim.Name, object_patch.WithSubresource("/status"))
	}
}

func (ipam *IPAM) loadAndProcessIPAddressLease(lease *VirtualMachineIPAddressLeaseSnapshot) error {
	var claim *VirtualMachineIPAddressClaimSnapshot
	if lease.ClaimName != "" {
		claimSnap := ipam.input.Snapshots[ipClaimsSnapshot]
		claim = getIPClaim(&claimSnap, lease.ClaimNamespace, lease.ClaimName)
	}
	if claim == nil || (claim.Address != "" && claim.Address != lease.Address) {
		// No claims found, we can remove lease
		ipam.input.PatchCollector.Delete(gv, "VirtualMachineIPAddressLease", "", lease.Name)
		return nil
	}
	// Allocate IP address
	if _, ok := ipam.allocatedIPs[lease.Address]; ok {
		return fmt.Errorf("Duplicated IP address lease %s", lease.Address)
	}
	ipam.allocatedIPs[lease.Address] = struct{}{}
	return nil
}

// Load CIDRs from config
func (ipam *IPAM) loadCIDRsFromConfig() error {
	for _, cidr := range ipam.input.Values.Get("virtualization.vmCIDRs").Array() {
		_, parsedCIDR, err := net.ParseCIDR(cidr.String())
		if err != nil || parsedCIDR == nil {
			return fmt.Errorf("Can not parse CIDR %s", cidr)
		}
		ipam.parsedCIDRs = append(ipam.parsedCIDRs, parsedCIDR)
	}
	return nil
}

func (ipam *IPAM) processIPAddressClaim(claim *VirtualMachineIPAddressClaimSnapshot) error {
	// Check for already allocated IP address
	_, alreadyAllocated := ipam.allocatedIPs[claim.Address]
	if alreadyAllocated && claim.Phase != "Bound" {
		ipam.handleConflictedClaim(claim)
		// Stop processing conflicting claim
		return nil
	}

	var lease *VirtualMachineIPAddressLeaseSnapshot
	if claim.LeaseName != "" {
		leaseSnap := ipam.input.Snapshots[ipLeasesSnapshot]
		lease = getIPLease(&leaseSnap, claim.LeaseName)
		if lease != nil && claim.Address != "" && claim.Address != lease.Address {
			// Wrong lease specified
			lease = nil
		}
	}

	if lease == nil {
		// Lease not found, create a new one
		lease = ipam.createLeaseForClaim(claim)
		if lease == nil {
			// Lease creation failed
			return nil
		}
	} else {
		// Lease found
		if !ipam.claimIsValidForLease(lease, claim) {
			ipam.handleConflictedClaim(claim)
			// Stop processing conflicting claim
			return nil
		}
		ipam.patchLeaseForClaim(lease, claim)
	}

	ipam.patchLeaseStatusForClaim(lease)
	ipam.patchClaimForLease(lease, claim)

	return nil
}
