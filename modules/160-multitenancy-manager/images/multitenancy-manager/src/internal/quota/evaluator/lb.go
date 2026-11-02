/*
Copyright 2026 Flant JSC

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

package evaluator

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// The keys of the lb family: the Services of type LoadBalancer of a project by their class, by their
// pool and by the external addresses they take.
const (
	// LBAddresses is the number of the external addresses of all the load balancers of a project.
	LBAddresses corev1.ResourceName = "loadbalancers.network.deckhouse.io/addresses"

	lbClassSuffix    = ".loadbalancerclass.network.deckhouse.io/"
	lbPoolSuffix     = ".loadbalancerpool.network.deckhouse.io/addresses"
	lbServicesKind   = "services"
	lbAddressesKind  = "addresses"
	lbDefaultClass   = "default"
	externalIPsCount = "network.deckhouse.io/l2-load-balancer-external-ips-count"
	loadBalancerIPs  = "network.deckhouse.io/load-balancer-ips"
	metalLBClass     = "network.deckhouse.io/metal-load-balancer-class"
	loadBalancerPool = "network.deckhouse.io/load-balancer-pool"
	// boundClassCondition is the condition of the status of a Service the metallb module puts the class
	// it serves the Service with into, as the message.
	boundClassCondition = "network.deckhouse.io/load-balancer-class"
)

// LBClassServices is the key of the number of the load balancers of the class.
func LBClassServices(class string) corev1.ResourceName {
	return corev1.ResourceName(class + lbClassSuffix + lbServicesKind)
}

// LBClassAddresses is the key of the number of the external addresses of the class.
func LBClassAddresses(class string) corev1.ResourceName {
	return corev1.ResourceName(class + lbClassSuffix + lbAddressesKind)
}

// LBPoolAddresses is the key of the number of the external addresses taken from the pool.
func LBPoolAddresses(pool string) corev1.ResourceName {
	return corev1.ResourceName(pool + lbPoolSuffix)
}

// IsLBKey reports whether the key belongs to the lb family.
func IsLBKey(key corev1.ResourceName) bool {
	name := string(key)
	if key == LBAddresses {
		return true
	}
	if class, kind, ok := strings.Cut(name, lbClassSuffix); ok {
		return validPart(class) && (kind == lbServicesKind || kind == lbAddressesKind)
	}
	if pool, ok := strings.CutSuffix(name, lbPoolSuffix); ok {
		return validPart(pool)
	}
	return false
}

func validPart(part string) bool {
	return part != "" && !strings.ContainsAny(part, "/")
}

// LoadBalancer is the family of the Services of type LoadBalancer. The class of a Service is the one
// the metallb module serves it with: the class its status is bound to, as it is, else
// spec.loadBalancerClass, else the network.deckhouse.io/metal-load-balancer-class annotation, each only
// when such a MetalLoadBalancerClass exists, else the MetalLoadBalancerClass marked default, else
// spec.loadBalancerClass as it is, the class of another controller, else "default", the load balancers
// of a cloud. Where the kind is not served, the class is spec.loadBalancerClass, else "default": no
// controller reads the annotation then. The external addresses are the
// network.deckhouse.io/l2-load-balancer-external-ips-count annotation when it is a number above one,
// else one, as the metallb module assigns them: network.deckhouse.io/load-balancer-ips only picks which
// addresses. A Service of a pool of the BGP mode takes one address.
type LoadBalancer struct {
	// DefaultClass is the MetalLoadBalancerClass marked default, empty when there is none.
	DefaultClass string
	// Classes are the names of the MetalLoadBalancerClass objects, nil when the kind is not served:
	// spec.loadBalancerClass is then taken as it is, and the annotation of the metallb module is not
	// read.
	Classes map[string]bool
}

// ClassesOf finds the MetalLoadBalancerClass marked default and the names of all of them.
func ClassesOf(classes []unstructured.Unstructured) (string, map[string]bool) {
	defaultClass := ""
	names := make(map[string]bool, len(classes))
	for i := range classes {
		names[classes[i].GetName()] = true
		if isDefault, _, _ := unstructured.NestedBool(classes[i].Object, "spec", "isDefault"); isDefault && defaultClass == "" {
			defaultClass = classes[i].GetName()
		}
	}
	return defaultClass, names
}

// Family is "lb".
func (LoadBalancer) Family() string { return "lb" }

// Usage is what the Service adds to the project; a Service of another type adds nothing.
func (e LoadBalancer) Usage(object runtime.Object) (corev1.ResourceList, error) {
	svc, ok := object.(*corev1.Service)
	if !ok {
		return nil, fmt.Errorf("the lb family evaluates Service objects, got %T", object)
	}
	usage := corev1.ResourceList{}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return usage, nil
	}
	annotations := svc.Annotations
	addresses := int64(1)
	pool := annotations[loadBalancerPool]
	if pool == "" {
		addresses = addressCount(annotations)
	}
	class := e.class(svc)
	one := *resource.NewQuantity(1, resource.DecimalSI)
	count := *resource.NewQuantity(addresses, resource.DecimalSI)
	usage[LBClassServices(class)] = one
	usage[LBClassAddresses(class)] = count
	usage[LBAddresses] = count.DeepCopy()
	if pool != "" {
		usage[LBPoolAddresses(pool)] = count.DeepCopy()
	}
	return usage, nil
}

// class mirrors how the metallb module picks the MetalLoadBalancerClass of a Service.
func (e LoadBalancer) class(svc *corev1.Service) string {
	for _, condition := range svc.Status.Conditions {
		if condition.Type == boundClassCondition && condition.Message != "" {
			return condition.Message
		}
	}
	specClass := ""
	if svc.Spec.LoadBalancerClass != nil {
		specClass = *svc.Spec.LoadBalancerClass
	}
	annotated := svc.Annotations[metalLBClass]
	if e.Classes == nil {
		// without the kind, the metallb module serves no Service, and the controller that does reads
		// spec.loadBalancerClass only
		switch {
		case specClass != "":
			return specClass
		case e.DefaultClass != "":
			return e.DefaultClass
		}
		return lbDefaultClass
	}
	switch {
	case specClass != "" && e.Classes[specClass]:
		return specClass
	case annotated != "" && e.Classes[annotated]:
		return annotated
	case e.DefaultClass != "":
		return e.DefaultClass
	case specClass != "":
		return specClass
	}
	return lbDefaultClass
}

// SumLoadBalancers adds up what the Services take. Every Service counts its own addresses, also one
// that shares an address under the network.deckhouse.io/load-balancer-shared-ip-key annotation.
func (e LoadBalancer) SumLoadBalancers(services []corev1.Service) corev1.ResourceList {
	total := corev1.ResourceList{}
	for i := range services {
		usage, err := e.Usage(&services[i])
		if err != nil {
			continue
		}
		for key, value := range usage {
			sum := total[key]
			sum.Add(value)
			total[key] = sum
		}
	}
	return total
}

// keptAnnotations are the annotations of a Service the family reads.
var keptAnnotations = []string{externalIPsCount, loadBalancerIPs, metalLBClass, loadBalancerPool}

// TrimService is the transform of an informer of Services: it keeps what the family counts a Service
// by and drops the rest, so that the Services of a large cluster take little memory.
func TrimService(object any) (any, error) {
	svc, ok := object.(*corev1.Service)
	if !ok {
		return object, nil
	}
	trimmed := &corev1.Service{
		TypeMeta: svc.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name: svc.Name, Namespace: svc.Namespace, UID: svc.UID, ResourceVersion: svc.ResourceVersion,
			DeletionTimestamp: svc.DeletionTimestamp,
		},
		Spec: corev1.ServiceSpec{Type: svc.Spec.Type, LoadBalancerClass: svc.Spec.LoadBalancerClass},
	}
	for _, key := range keptAnnotations {
		if value, ok := svc.Annotations[key]; ok {
			if trimmed.Annotations == nil {
				trimmed.Annotations = map[string]string{}
			}
			trimmed.Annotations[key] = value
		}
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return trimmed, nil
	}
	for _, condition := range svc.Status.Conditions {
		if condition.Type == boundClassCondition {
			trimmed.Status.Conditions = append(trimmed.Status.Conditions, metav1.Condition{Type: condition.Type, Status: condition.Status, Message: condition.Message})
		}
	}
	return trimmed, nil
}

// addressCount is the number of the external addresses a Service of the L2 mode asks for. The
// metallb module takes a count that is not a number above one as one, and so does the ledger; the
// addresses that network.deckhouse.io/load-balancer-ips lists are only the ones it picks first.
func addressCount(annotations map[string]string) int64 {
	if count, err := strconv.ParseInt(annotations[externalIPsCount], 10, 64); err == nil && count > 1 {
		return count
	}
	return 1
}
