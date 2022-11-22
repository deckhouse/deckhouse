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

	"k8s.io/utils/strings/slices"

	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func unstructuredToNodeGroup(o *unstructured.Unstructured) (*NodeGroup, error) {
	content, err := o.MarshalJSON()
	if err != nil {
		log.ErrorF("Can not marshal nodegroup %s: %v", o.GetName(), err)
		return nil, err
	}

	var ng NodeGroup

	err = json.Unmarshal(content, &ng)
	if err != nil {
		log.ErrorF("Can not unmarshal nodegroup %s: %v", o.GetName(), err)
		return nil, err
	}

	return &ng, nil
}

type nodeGroupGetter interface {
	NodeGroup(string) (*NodeGroup, error)
	Events(string) ([]eventsv1.Event, error)
}

type kubeNodegroupGetter struct {
	kubeCl *client.KubernetesClient
}

func (n *kubeNodegroupGetter) Events(ng string) ([]eventsv1.Event, error) {
	list, err := n.kubeCl.EventsV1().Events("default").List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("regarding.name=%s", ng),
		TypeMeta:      metav1.TypeMeta{Kind: "NodeGroup", APIVersion: "deckhouse.io/v1"},
	})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (n *kubeNodegroupGetter) NodeGroup(ngName string) (*NodeGroup, error) {
	unstruct, err := converge.GetNodeGroup(n.kubeCl, ngName)
	if err != nil {
		return nil, err
	}

	return unstructuredToNodeGroup(unstruct)
}

type nodegroupChecker struct {
	ngGetter nodeGroupGetter
	ngName   string
	logger   log.Logger

	startCheckTime time.Time
}

func newNodeGroupChecker(ngGetter nodeGroupGetter, ngName string) *nodegroupChecker {
	return &nodegroupChecker{
		ngGetter: ngGetter,
		ngName:   ngName,
		logger:   log.GetDefaultLogger(),

		startCheckTime: time.Now().Add(1 * time.Minute),
	}
}

func (n *nodegroupChecker) withLogger(l log.Logger) *nodegroupChecker {
	n.logger = l
	return n
}

func (n *nodegroupChecker) lastEvents(lastTime time.Duration, reason string) ([]eventsv1.Event, error) {
	events, err := n.ngGetter.Events(n.ngName)
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
			if reason != "" && e.Reason != reason {
				continue
			}

			res = append(res, e)
			continue
		}

		break
	}

	return res, nil
}

func (n *nodegroupChecker) IsReady() (bool, error) {
	ng, err := n.ngGetter.NodeGroup(n.ngName)
	if err != nil {
		return false, err
	}

	n.logger.LogInfoF("For node group '%s':\n", n.ngName)

	// todo ask
	if ng.Status.Desired > 0 && ng.Status.Ready == ng.Status.Desired {
		n.logger.LogInfoF("\t\tAll nodes are 'Ready'\n")
		return true, nil
	}

	if time.Now().Before(n.startCheckTime) {
		n.logger.LogInfoF("\t\tWaiting 1 minute for stabilize node group status\n")
		return false, nil
	}

	if ng.Status.Desired == 0 {
		n.logger.LogInfoF("\t\tWaiting for desired nodes will be greater than 0\n")
		return false, nil
	}

	if len(ng.Status.ConditionSummary.StatusMessage) > 0 {
		n.logger.LogErrorF("\t\tLast machine failures:\n %s\n", ng.Status.ConditionSummary.StatusMessage)
		for _, f := range ng.Status.LastMachineFailures {
			n.logger.LogErrorF("\t\t\t%s\n", f.LastOperation.Description)
		}

		dur := 1 * time.Minute
		events, err := n.lastEvents(dur, "MachineFailed")
		if err != nil {
			return false, err
		}

		if len(events) > 0 {
			n.logger.LogErrorF("\t\tLast events for node group:\n", dur.String())
			for _, e := range events {
				n.logger.LogErrorF("\t\t\t%s: %s\n", e.Reason, e.Note)
			}
		}

		return false, nil
	}

	n.logger.LogInfoF("\t\tWaiting for 'Ready' nodes count will be equal 'Desired' nodes count (%d/%d)\n",
		ng.Status.Ready, ng.Status.Desired)

	return false, nil
}

func (n *nodegroupChecker) Name() string {
	return fmt.Sprintf("NodeGroup %s readiness check", n.ngName)
}

func tryToGetEphemeralNodeGroupChecker(
	kubeCl *client.KubernetesClient,
	r *template.Resource,
	skipNG []string) (*nodegroupChecker, error) {
	if !(r.GVK.Kind == "NodeGroup" && r.GVK.Group == "deckhouse.io" && r.GVK.Version == "v1") {
		log.DebugF("tryToGetEphemeralNodeGroupChecker: skip GVK (%s %s %s)",
			r.GVK.Version, r.GVK.Group, r.GVK.Kind)
		return nil, nil
	}

	if slices.Contains(skipNG, r.Object.GetName()) {
		log.DebugF("Nodegroup %s was skipped", r.Object.GetName())
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

	log.InfoF("Got readiness checker for nodegroup %s", ng.GetName())

	return newNodeGroupChecker(&kubeNodegroupGetter{kubeCl: kubeCl}, ng.GetName()), nil
}
