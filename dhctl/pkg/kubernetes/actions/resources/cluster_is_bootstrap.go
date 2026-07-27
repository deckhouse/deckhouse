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
	"fmt"
	"sort"
	"strings"
	"time"

	multierr "github.com/hashicorp/go-multierror"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type nodeGroupGetter interface {
	NodeGroups(ctx context.Context) ([]*v1.NodeGroup, error)
	MachineFailedEvents(ctx context.Context) ([]eventsv1.Event, error)
}

type kubeNgGetter struct {
	kubeProvider kubernetes.KubeClientProviderWithCtx
}

func (n *kubeNgGetter) NodeGroups(ctx context.Context) ([]*v1.NodeGroup, error) {
	var ngs []unstructured.Unstructured
	err := retry.NewSilentLoop("get machine failed events", 9, 1*time.Second).RunContext(ctx, func() error {
		kubeCl, err := n.kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}
		ngs, err = entity.GetNodeGroups(ctx, kubeCl)
		return err
	})
	if err != nil {
		return nil, err
	}

	nodegroups := make([]*v1.NodeGroup, 0)
	var errs error
	for _, n := range ngs {
		ng, err := entity.UnstructuredToNodeGroup(new(n))
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

func (n *kubeNgGetter) MachineFailedEvents(ctx context.Context) ([]eventsv1.Event, error) {
	var list *eventsv1.EventList
	err := retry.NewSilentLoop("get machine failed events", 9, 1*time.Second).RunContext(ctx, func() error {
		kubeCl, err := n.kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}

		list, err = kubeCl.EventsV1().Events("default").List(ctx, metav1.ListOptions{
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
	ngGetter     nodeGroupGetter
	kubeProvider kubernetes.KubeClientProviderWithCtx

	startCheckTime time.Time
	attempts       int32
}

func newClusterIsBootstrapCheck(ngGetter nodeGroupGetter, params constructorParams) *clusterIsBootstrapCheck {
	return &clusterIsBootstrapCheck{
		ngGetter:     ngGetter,
		kubeProvider: params.kubeProvider,

		startCheckTime: time.Now().Add(1 * time.Minute),
		// start from 1 for prevent output table at first time because
		// we can get false positive error: "Wrong classReference: There is no valid instance class CLASS_NAME of
		// type *InstanceClass"
		attempts: 1,
	}
}

func (n *clusterIsBootstrapCheck) lastEvents(ctx context.Context, lastTime time.Duration) ([]eventsv1.Event, error) {
	events, err := n.ngGetter.MachineFailedEvents(ctx)
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

func (n *clusterIsBootstrapCheck) hasBootstrappedCM(ctx context.Context) (bool, error) {
	hasCm := false
	err := retry.NewSilentLoop("get is-bootstrapped cm", 9, 1*time.Second).RunContext(ctx, func() error {
		kubeCl, err := n.kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}

		_, err = kubeCl.CoreV1().ConfigMaps("kube-system").
			Get(ctx, "d8-cluster-is-bootstraped", metav1.GetOptions{})
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

func (n *clusterIsBootstrapCheck) outputNodeGroups(ctx context.Context) string {
	if n.attempts%4 != 0 {
		return ""
	}

	ngs, err := n.ngGetter.NodeGroups(ctx)
	if err != nil {
		return ""
	}

	if len(ngs) == 0 {
		return ""
	}

	fs := "%-30s %-8s %-8s %-9s %-8s %-17s\n"
	var out strings.Builder
	fmt.Fprintf(&out, fs, "NAME", "READY", "NODES", "INSTANCES", "DESIRED", "STATUS")
	for _, ng := range ngs {
		stat := ng.Status
		o := fmt.Sprintf(fs,
			ng.Name,
			fmt.Sprint(stat.Ready),
			fmt.Sprint(stat.Nodes),
			fmt.Sprint(stat.Instances),
			fmt.Sprint(stat.Desired),
			stat.Error)
		out.WriteString(o)
	}

	return strings.TrimSuffix(out.String(), "\n")
}

func (n *clusterIsBootstrapCheck) outputMachineFailures(ctx context.Context) {
	if time.Now().Before(n.startCheckTime) {
		dhlog.FromContext(ctx).DebugContext(ctx, "Waiting 1 minute for stabilizing node group events")
		return
	}

	events, err := n.lastEvents(ctx, 1*time.Minute)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Error while getting last events: %v", err), "\n"))
		return
	}

	if len(events) == 0 {
		return
	}

	dhlog.FromContext(ctx).ErrorContext(ctx, "\nMachine Failures:")
	for _, e := range events {
		dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("\t%s", e.Note))
	}
}

func (n *clusterIsBootstrapCheck) Name() string {
	return "cluster"
}

func (n *clusterIsBootstrapCheck) ReadyMsg() string {
	return "The cluster is bootstrapped."
}

func (n *clusterIsBootstrapCheck) Single() bool {
	return true
}

func (n *clusterIsBootstrapCheck) IsReady(ctx context.Context) (bool, error) {
	defer func() {
		n.attempts++
	}()

	ok, err := n.hasBootstrappedCM(ctx)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error while checking cluster state: %v", err))
		return false, nil
	}

	if ok {
		return true, nil
	}

	if len(n.outputNodeGroups(ctx)) > 0 {
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "NodeGroups status", func(ctx context.Context) error {
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprint(n.outputNodeGroups(ctx)))
			return nil
		})
	}

	n.outputMachineFailures(ctx)

	return false, nil
}

func tryToGetClusterIsBootstrappedChecker(ctx context.Context, r *template.Resource, params constructorParams) (Checker, error) {
	if r.GVK.Kind != "NodeGroup" || r.GVK.Group != "deckhouse.io" || r.GVK.Version != "v1" {
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("tryToGetClusterIsBootstrappedChecker: skip GVK (%s %s %s)",
			r.GVK.Version, r.GVK.Group, r.GVK.Kind), "\n"))
		return nil, nil
	}

	ng, err := entity.UnstructuredToNodeGroup(&r.Object)
	if err != nil {
		return nil, err
	}

	if ng.Spec.NodeType != "CloudEphemeral" {
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Skip nodegroup %s, because type %s is not supported", ng.GetName(), ng.Spec.NodeType), "\n"))
		return nil, nil
	}

	if ng.Spec.CloudInstances.MinPerZone == nil || ng.Spec.CloudInstances.MaxPerZone == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Skip nodegroup %s, because type min and max per zone is not set", ng.GetName()), "\n"))
		return nil, nil
	}

	if *ng.Spec.CloudInstances.MinPerZone < 0 || *ng.Spec.CloudInstances.MaxPerZone < 1 {
		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Skip nodegroup %s, because type min (%d) and max (%d) per zone is incorrect",
			ng.GetName(), *ng.Spec.CloudInstances.MinPerZone, *ng.Spec.CloudInstances.MaxPerZone), "\n"))
		return nil, nil
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Got readiness checker for nodegroup %s", ng.GetName()))

	ngGetter := &kubeNgGetter{kubeProvider: params.kubeProvider}
	return newClusterIsBootstrapCheck(ngGetter, params), nil
}

func tryToGetClusterIsBootstrappedCheckerFromStaticNGS(params constructorParams) Checker {
	if params.metaConfig == nil {
		return nil
	}

	for _, terraNg := range params.metaConfig.GetTerraNodeGroups() {
		if terraNg.Replicas > 0 {
			checker := newClusterIsBootstrapCheck(&kubeNgGetter{kubeProvider: params.kubeProvider}, params)
			return checker
		}
	}

	return nil
}
