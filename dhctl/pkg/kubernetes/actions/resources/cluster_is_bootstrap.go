// Copyright 2022 Flant JSC
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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	multierr "github.com/hashicorp/go-multierror"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type nodeGroupGetter interface {
	NodeGroups() ([]*v1.NodeGroup, error)
	MachineFailedEvents() ([]eventsv1.Event, error)
}

type kubeNgGetter struct {
	kubeCl *client.KubernetesClient
}

func (n *kubeNgGetter) NodeGroups() ([]*v1.NodeGroup, error) {
	var ngs []unstructured.Unstructured
	err := retry.NewSilentLoop("get machine failed events", 3, 3*time.Second).Run(func() error {
		var err error
		ngs, err = converge.GetNodeGroups(n.kubeCl)
		return err
	})

	if err != nil {
		return nil, err
	}

	nodegroups := make([]*v1.NodeGroup, 0)
	var errs error
	for _, n := range ngs {
		nn := n
		ng, err := unstructuredToNodeGroup(&nn)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		nodegroups = append(nodegroups, ng)
	}

	if errs != nil {
		return nil, errs
	}

	return nodegroups, err
}

func (n *kubeNgGetter) MachineFailedEvents() ([]eventsv1.Event, error) {
	var list *eventsv1.EventList
	err := retry.NewSilentLoop("get machine failed events", 3, 3*time.Second).Run(func() error {
		var err error
		list, err = n.kubeCl.EventsV1().Events("default").List(context.TODO(), metav1.ListOptions{
			FieldSelector: "reason=MachineFailed",
			TypeMeta:      metav1.TypeMeta{Kind: "NodeGroup", APIVersion: "deckhouse.io/v1"},
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

type clusterIsBootstrapCheck struct {
	ngGetter nodeGroupGetter
	logger   log.Logger
	kubeCl   *client.KubernetesClient

	startCheckTime time.Time
	attempts       int32
}

func newClusterIsBootstrapCheck(ngGetter nodeGroupGetter, kubeCl *client.KubernetesClient) *clusterIsBootstrapCheck {
	return &clusterIsBootstrapCheck{
		ngGetter: ngGetter,
		kubeCl:   kubeCl,
		logger:   log.GetDefaultLogger(),

		startCheckTime: time.Now().Add(1 * time.Minute),
		// start from 1 for prevent output table at first time because
		// we can get false positive error: "Wrong classReference: There is no valid instance class CLASS_NAME of
		// type *InstanceClass"
		attempts: 1,
	}
}

func (n *clusterIsBootstrapCheck) lastEvents(lastTime time.Duration) ([]eventsv1.Event, error) {
	events, err := n.ngGetter.MachineFailedEvents()
	if err != nil {
		return nil, err
	}

	sort.Slice(events, func(i, j int) bool {
		// sort reverse
		return events[j].ObjectMeta.CreationTimestamp.Before(&events[i].ObjectMeta.CreationTimestamp)
	})

	tt := time.Now().Add(-lastTime)
	res := make([]eventsv1.Event, 0)
	for _, e := range events {
		if e.ObjectMeta.CreationTimestamp.After(tt) {
			res = append(res, e)
			continue
		}

		break
	}

	return res, nil
}

func (n *clusterIsBootstrapCheck) hasBootstrappedCM() (bool, error) {
	hasCm := false
	err := retry.NewSilentLoop("get is-bootstrapped cm", 3, 3*time.Second).Run(func() error {
		_, err := n.kubeCl.CoreV1().ConfigMaps("kube-system").
			Get(context.TODO(), "d8-cluster-is-bootstraped", metav1.GetOptions{})
		if err == nil {
			hasCm = true
			return nil
		}

		if errors.IsNotFound(err) {
			hasCm = false
			return nil
		}

		return err
	})

	return hasCm, err
}

func (n *clusterIsBootstrapCheck) outputNodeGroups() {
	if n.attempts%4 != 0 {
		return
	}

	ngs, err := n.ngGetter.NodeGroups()
	if err != nil {
		n.logger.LogDebugF("Error while getting node groups: %v", err)
		return
	}

	if len(ngs) == 0 {
		return
	}

	fs := "%-30s %-8s %-8s %-9s %-8s %-17s\n"
	n.logger.LogInfoF(fs, "NAME", "READY", "NODES", "INSTANCES", "DESIRED", "STATUS")
	for _, ng := range ngs {
		stat := ng.Status
		n.logger.LogInfoF(fs,
			ng.Name,
			fmt.Sprint(stat.Ready),
			fmt.Sprint(stat.Nodes),
			fmt.Sprint(stat.Instances),
			fmt.Sprint(stat.Desired),
			stat.Error)
	}
}

func (n *clusterIsBootstrapCheck) outputMachineFailures() {
	if time.Now().Before(n.startCheckTime) {
		n.logger.LogDebugF("Waiting 1 minute for stabilizing node group events\n")
		return
	}

	events, err := n.lastEvents(1 * time.Minute)
	if err != nil {
		n.logger.LogDebugF("Error while getting last events: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	n.logger.LogErrorF("\nMachine Failures:\n")
	for _, e := range events {
		n.logger.LogErrorF("\t%s\n", e.Note)
	}
}

func (n *clusterIsBootstrapCheck) Name() string {
	return "Waiting for the cluster to become bootstrapped."
}

func (n *clusterIsBootstrapCheck) Single() bool {
	return true
}

func (n *clusterIsBootstrapCheck) IsReady() (bool, error) {
	defer func() {
		n.attempts++
		n.logger.LogInfoF("\n")
	}()

	n.logger.LogInfoF("Waiting for the cluster to be in the 'bootstrapped' state:\n")

	notBootstrappedMsg := "The cluster has not been bootstrapped yet. Waiting for at least one non-master node in Ready status.\n"

	ok, err := n.hasBootstrappedCM()
	if err != nil {
		n.logger.LogDebugF("Error while checking cluster state: %v\n", err)
		n.logger.LogInfoF(notBootstrappedMsg)
		return false, nil
	}

	if ok {
		n.logger.LogInfoF("The cluster is bootstrapped. Waiting for the creation of resources.\n")
		return true, nil
	}

	n.logger.LogInfoF(notBootstrappedMsg)

	n.outputNodeGroups()

	n.outputMachineFailures()

	return false, nil
}

func tryToGetClusterIsBootstrappedChecker(
	kubeCl *client.KubernetesClient,
	_ *config.MetaConfig,
	r *template.Resource) (Checker, error) {
	if !(r.GVK.Kind == "NodeGroup" && r.GVK.Group == "deckhouse.io" && r.GVK.Version == "v1") {
		log.DebugF("tryToGetClusterIsBootstrappedChecker: skip GVK (%s %s %s)",
			r.GVK.Version, r.GVK.Group, r.GVK.Kind)
		return nil, nil
	}

	ng, err := unstructuredToNodeGroup(&r.Object)
	if err != nil {
		return nil, err
	}

	if ng.Spec.NodeType != "CloudEphemeral" {
		log.DebugF("Skip nodegroup %s, because type %s is not supported", ng.GetName(), ng.Spec.NodeType)
		return nil, nil
	}

	if ng.Spec.CloudInstances.MinPerZone == nil || ng.Spec.CloudInstances.MaxPerZone == nil {
		log.DebugF("Skip nodegroup %s, because type min and max per zone is not set", ng.GetName())
		return nil, nil
	}

	if *ng.Spec.CloudInstances.MinPerZone < 0 || *ng.Spec.CloudInstances.MaxPerZone < 1 {
		log.DebugF("Skip nodegroup %s, because type min (%d) and max (%d) per zone is incorrect",
			ng.GetName(), *ng.Spec.CloudInstances.MinPerZone, *ng.Spec.CloudInstances.MaxPerZone)
		return nil, nil
	}

	log.DebugF("Got readiness checker for nodegroup %s\n", ng.GetName())
	return newClusterIsBootstrapCheck(&kubeNgGetter{kubeCl: kubeCl}, kubeCl), nil
}

func unstructuredToNodeGroup(o *unstructured.Unstructured) (*v1.NodeGroup, error) {
	content, err := o.MarshalJSON()
	if err != nil {
		log.ErrorF("Can not marshal nodegroup %s: %v", o.GetName(), err)
		return nil, err
	}

	var ng v1.NodeGroup

	err = json.Unmarshal(content, &ng)
	if err != nil {
		log.ErrorF("Can not unmarshal nodegroup %s: %v", o.GetName(), err)
		return nil, err
	}

	return &ng, nil
}

func tryToGetClusterIsBootstrappedCheckerFromStaticNGS(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (Checker, error) {
	if metaConfig == nil {
		return nil, nil
	}

	for _, terraNg := range metaConfig.GetTerraNodeGroups() {
		if terraNg.Replicas > 0 {
			checker := newClusterIsBootstrapCheck(&kubeNgGetter{kubeCl: kubeCl}, kubeCl)
			return checker, nil
		}
	}

	return nil, nil
}
