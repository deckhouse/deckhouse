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

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type nodeGroupGetter interface {
	NodeGroups() ([]*NodeGroup, error)
	MachineFailedEvents() ([]eventsv1.Event, error)
}

type kubeNgGetter struct {
	kubeCl *client.KubernetesClient
}

func (n *kubeNgGetter) NodeGroups() ([]*NodeGroup, error) {
	var ngs []unstructured.Unstructured
	err := retry.NewSilentLoop("get machine failed events", 3, 3*time.Second).Run(func() error {
		var err error
		ngs, err = converge.GetNodeGroups(n.kubeCl)
		return err
	})

	if err != nil {
		return nil, err
	}

	nodegroups := make([]*NodeGroup, 0)
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
			FieldSelector: fmt.Sprintf("reason=MachineFailed"),
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
}

func newClusterIsBootstrapCheck(ngGetter nodeGroupGetter, kubeCl *client.KubernetesClient) *clusterIsBootstrapCheck {
	return &clusterIsBootstrapCheck{
		ngGetter: ngGetter,
		kubeCl:   kubeCl,
		logger:   log.GetDefaultLogger(),

		startCheckTime: time.Now().Add(1 * time.Minute),
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

func (n *clusterIsBootstrapCheck) outputNodeGroups() bool {
	ngs, err := n.ngGetter.NodeGroups()
	if err != nil {
		n.logger.LogErrorF("Error while getting node groups: %v", err)
		return false
	}

	if len(ngs) == 0 {
		return false
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

	return true
}

func (n *clusterIsBootstrapCheck) outputMachineFailures() bool {
	if time.Now().Before(n.startCheckTime) {
		n.logger.LogDebugF("Waiting 1 minute for stabilize node group events\n")
		return false
	}

	events, err := n.lastEvents(1 * time.Minute)
	if err != nil {
		n.logger.LogErrorF("Error while getting node groups: %v", err)
		return false
	}

	if len(events) == 0 {
		return true
	}

	n.logger.LogErrorF("\nMachine Failures:\n")
	for _, e := range events {
		n.logger.LogErrorF("\t%s\n", e.Note)

	}

	return true
}

func (n *clusterIsBootstrapCheck) Name() string {
	return "Waiting for cluster is bootstrapped"
}

func (n *clusterIsBootstrapCheck) IsReady() (bool, error) {
	defer func() {
		n.logger.LogInfoF("\n")
	}()

	n.logger.LogInfoF("Waiting for cluster will be in 'bootstrapped' state:\n")

	ok, err := n.hasBootstrappedCM()
	if err != nil {
		n.logger.LogErrorF("Error while checking cluster state: %v", err)
		return false, nil
	}

	if ok {
		n.logger.LogInfoF("Cluster is bootstrapped. Waiting for resource creation.\n")
		return true, nil
	}

	n.logger.LogInfoF("Cluster is not yet bootstrapped. Waiting.\n")

	outEvents := n.outputNodeGroups()

	if outEvents {
		n.outputMachineFailures()
	}

	return false, nil
}

func tryToGetClusterIsBootstrappedChecker(
	kubeCl *client.KubernetesClient,
	r *template.Resource) (*clusterIsBootstrapCheck, error) {
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
