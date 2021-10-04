/*
Copyright 2021 Flant JSC

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

package smokemini

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

var _ = Describe("Modules :: upmeter :: hooks :: smokemini :: pvc ::", func() {
	pvc := func(x string, terminating bool) snapshot.PvcTermination {
		return snapshot.PvcTermination{
			Name:          snapshot.Index(x).PersistenceVolumeClaimName(),
			IsTerminating: terminating,
		}
	}

	pod := func(x string, pending bool) snapshot.PodPhase {
		return snapshot.PodPhase{
			Name:      snapshot.Index(x).PodName(),
			IsPending: pending,
		}
	}

	pvcOk := func(x string) snapshot.PvcTermination { return pvc(x, false) }
	pvcNotOk := func(x string) snapshot.PvcTermination { return pvc(x, true) }

	podOk := func(x string) snapshot.PodPhase { return pod(x, false) }
	podNotOk := func(x string) snapshot.PodPhase { return pod(x, true) }

	table.DescribeTable("indexesForDeletion", func(pvcs []snapshot.PvcTermination, pods []snapshot.PodPhase, expected set.Set) {
		s := indexesForDeletion(pvcs, pods)

		for x := range expected {
			Expect(s.Has(x)).To(BeTrue(), "index %q should be returned", x)
			s.Delete(x)
		}

		Expect(s.Size()).To(Equal(0), "indexes %v should not be returned", s.Slice())
	},
		table.Entry("empty inputs result in no deletions", nil, nil, set.New()),
		table.Entry("single healthy Pod and PVC result in no deletions",
			[]snapshot.PvcTermination{pvcOk("a")},
			[]snapshot.PodPhase{podOk("a")},
			set.New(),
		),
		table.Entry("pending Pod and healthy PVC result in deletion",
			[]snapshot.PvcTermination{pvcOk("a")},
			[]snapshot.PodPhase{podNotOk("a")},
			set.New("a"),
		),
		table.Entry("healthy Pod and terminating PVC result in deletion",
			[]snapshot.PvcTermination{pvcNotOk("a")},
			[]snapshot.PodPhase{podOk("a")},
			set.New("a"),
		),
		table.Entry("all healthy Pods and PVCs result in no deletions",
			[]snapshot.PvcTermination{
				pvcOk("a"), pvcOk("b"), pvcOk("c"), pvcOk("d"), pvcOk("e"),
			},
			[]snapshot.PodPhase{
				podOk("a"), podOk("b"), podOk("c"), podOk("d"), podOk("e"),
			},
			set.New(),
		),
		table.Entry("some pending Pods and helthy PVCs result in deletions of the pods",
			[]snapshot.PvcTermination{
				pvcOk("a"), pvcOk("b"), pvcOk("c"), pvcOk("d"), pvcOk("e"),
			},
			[]snapshot.PodPhase{
				podOk("a"), podNotOk("b"),
				podOk("c"), podNotOk("d"),
				podOk("e"),
			},
			set.New("b", "d"),
		),
		table.Entry("all running Pods and some terminating PVCs result in deletions of the pods",
			[]snapshot.PvcTermination{
				pvcOk("a"), pvcNotOk("b"),
				pvcOk("c"), pvcNotOk("d"),
				pvcOk("e"),
			},
			[]snapshot.PodPhase{
				podOk("a"), podOk("b"), podOk("c"), podOk("d"), podOk("e"),
			},
			set.New("b", "d"),
		),
		table.Entry("absent pvc result in deletions of the pods",
			[]snapshot.PvcTermination{
				pvcOk("a"), pvcOk("c"), pvcOk("e"),
			},
			[]snapshot.PodPhase{
				podOk("a"), podOk("b"), podOk("c"), podOk("d"), podOk("e"),
			},
			set.New("b", "d"),
		),
		table.Entry("absent Pods for terminating PVC are not added to deletions",
			[]snapshot.PvcTermination{
				pvcOk("a"), pvcNotOk("b"),
				pvcOk("c"), pvcNotOk("d"),
				pvcOk("e"),
			},
			[]snapshot.PodPhase{
				podOk("a"), /* no b */
				podOk("c"), podNotOk("d"),
				podOk("e"),
			},
			set.New("d"),
		),
	)
})
