/*
Copyright 2022 Flant JSC

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
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	preemtibleVMDeletionDuration = 24 * time.Hour
)

type Machine struct {
	Name              string
	CreationTimestamp metav1.Time
	Terminating       bool
	MachineClassKind  string
	MachineClassName  string
}

type YandexMachineClass struct {
	Name string
}

func applyMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var terminating bool
	if obj.GetDeletionTimestamp() != nil {
		terminating = true
	}

	classKind, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "class", "kind")
	if err != nil {
		return nil, fmt.Errorf("can't access class name of Machine %q: %s", obj.GetName(), err)
	}
	if len(classKind) == 0 {
		return nil, fmt.Errorf("spec.class.kind is empty in %q", obj.GetName())
	}

	className, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "class", "name")
	if err != nil {
		return nil, fmt.Errorf("can't access class name of Machine %q: %s", obj.GetName(), err)
	}
	if len(className) == 0 {
		return nil, fmt.Errorf("spec.class.name is empty in %q", obj.GetName())
	}

	return &Machine{
		Name:              obj.GetName(),
		CreationTimestamp: obj.GetCreationTimestamp(),
		Terminating:       terminating,
		MachineClassKind:  classKind,
		MachineClassName:  className,
	}, nil
}

func isPreemptibleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	preemptible, ok, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "schedulingPolicy", "preemptible")
	if err != nil {
		return nil, fmt.Errorf("can't access field \"spec.schedulingPolicy.preemptible\" of YandexMachineClass %q: %s", obj.GetName(), err)
	}

	if ok && preemptible {
		return &YandexMachineClass{
			Name: obj.GetName(),
		}, nil
	}

	return nil, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-yandex/preemtibly-delete-preemtible-instances",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "every-15",
			Crontab: "0/15 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "mcs",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "machine.sapcloud.io/v1alpha1",
			Kind:                "YandexMachineClass",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: isPreemptibleFilter,
		},
		{
			Name:                "machines",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "machine.sapcloud.io/v1alpha1",
			Kind:                "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: applyMachineFilter,
		},
	},
}, deleteMachines)

func deleteMachines(input *go_hook.HookInput) error {
	var (
		timeNow                      = time.Now().UTC()
		preemptibleMachineClassesSet = set.Set{}
		machines                     []*Machine
	)

	for _, mcRaw := range input.Snapshots["mcs"] {
		if mcRaw == nil {
			continue
		}

		ic, ok := mcRaw.(*YandexMachineClass)
		if !ok {
			return fmt.Errorf("failed to assert to *YandexMachineClass")
		}

		preemptibleMachineClassesSet.Add(ic.Name)
	}

	if preemptibleMachineClassesSet.Size() == 0 {
		return nil
	}

	for _, machineRaw := range input.Snapshots["machines"] {
		machine, ok := machineRaw.(*Machine)
		if !ok {
			return fmt.Errorf("failed to assert to *Machine")
		}

		if machine.Terminating {
			continue
		}

		if machine.MachineClassKind != "YandexMachineClass" {
			continue
		}

		if !preemptibleMachineClassesSet.Has(machine.MachineClassName) {
			continue
		}

		machines = append(machines, machine)
	}

	if len(machines) == 0 {
		return nil
	}

	for _, m := range getMachinesToDelete(timeNow, machines) {
		input.PatchCollector.Delete("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", m)
	}

	return nil
}

// delete all after 23h mark
// afterwards delete in 15 minutes increments, no more than batch size
func getMachinesToDelete(timeNow time.Time, machines []*Machine) (machinesToDelete []string) {
	const (
		// 12 * 0.25 = 3 hours
		durationIterations = 12
		slidingStep        = 15 * time.Minute
	)
	var (
		currentSlidingDuration = preemtibleVMDeletionDuration - time.Hour
	)

	sort.Slice(machines, func(i, j int) bool {
		return machines[i].CreationTimestamp.Before(&machines[j].CreationTimestamp)
	})

	batch := len(machines) / durationIterations
	if batch == 0 {
		batch = 1
	}

	var (
		cursor int
	)

	// short-circuit if there are Machines older than 23 hours
	for _, m := range machines {
		if expires(timeNow, m.CreationTimestamp.Time, currentSlidingDuration) {
			machinesToDelete = append(machinesToDelete, m.Name)
			cursor++
		}
	}
	if len(machinesToDelete) != 0 {
		return machinesToDelete
	}

	for t := 0; t < durationIterations; t++ {
		currentSlidingDuration -= slidingStep

		for cursor < len(machines) {
			if len(machinesToDelete) >= batch {
				break
			}

			if expires(timeNow, machines[cursor].CreationTimestamp.Time, currentSlidingDuration) {
				machinesToDelete = append(machinesToDelete, machines[cursor].Name)
				cursor++
			} else {
				break
			}
		}
	}

	return
}

func expires(now, timestamp time.Time, expirationDuration time.Duration) bool {
	return timestamp.Add(expirationDuration).Before(now)
}
