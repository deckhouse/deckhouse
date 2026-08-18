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

package v2

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/checker"
)

// found returns the named check, failing the test when the report does not contain it: a
// migration report that quietly stopped answering one of the five would otherwise pass every
// assertion made about the others.
func found(t *testing.T, checks []preflightCheck, name string) preflightCheck {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	require.Failf(t, "the report says nothing about a check", "check %q", name)
	return preflightCheck{}
}

// A green cluster: settled outside Local, a reachable registry, the new components' images
// resident everywhere, and nothing an operator wrote for the nodes.
func ready() preflight {
	return preflight{
		HasLegacy:       true,
		Legacy:          legacyState{Mode: "Direct"},
		CheckerReported: true,
		CheckerStatus:   checker.Status{Ready: true},
		ImageHolder: &imageHolderState{
			Images: []string{
				"registry.example.com/system/deckhouse@sha256:aaa",
				"registry.example.com/system/deckhouse@sha256:bbb",
				"registry.example.com/system/deckhouse@sha256:ccc",
			},
			Desired:   3,
			Available: 3,
		},
		RequiredImages: map[string]string{
			"dockerDistribution": "sha256:aaa",
			"dockerAuth":         "sha256:bbb",
			"registrySyncer":     "sha256:ccc",
		},
	}
}

func TestAReadyClusterPassesEveryCheck(t *testing.T) {
	checks := ready().report()

	require.Len(t, checks, 5, "every check has to be reported, including the ones that pass")
	for _, check := range checks {
		assert.Truef(t, check.Passed, "%s failed: %s", check.Name, check.Detail)
	}
}

// The two hard stops, and only those two. Blocking says the migration must not begin at all, so
// which checks carry it is part of the contract rather than an implementation detail.
func TestOnlyLocalAndAnUnreachableRegistryBlockTheMigration(t *testing.T) {
	cases := []struct {
		name     string
		subject  preflight
		check    string
		blocking bool
		detail   string
	}{{
		// The cluster whose registry is the cluster. Unmanaged would point every node at an
		// upstream it does not have.
		name: "in Local",
		subject: func() preflight {
			subject := ready()
			subject.Legacy = legacyState{Mode: "Local"}
			return subject
		}(),
		check:    CheckNotLocal,
		blocking: true,
		detail:   "fallback runbook",
	}, {
		name: "the registry does not answer",
		subject: func() preflight {
			subject := ready()
			subject.CheckerStatus = checker.Status{Ready: false, Message: "dial tcp: i/o timeout"}
			return subject
		}(),
		check:    CheckUpstreamReachable,
		blocking: true,
		detail:   "dial tcp: i/o timeout",
	}, {
		// Never asked is not the same as answered "no", and an operator reading this has to be
		// able to tell them apart before deciding.
		name: "the checker has not reported yet",
		subject: func() preflight {
			subject := ready()
			subject.CheckerReported = false
			subject.CheckerStatus = checker.Status{Ready: true}
			return subject
		}(),
		check:    CheckUpstreamReachable,
		blocking: true,
		detail:   "has not reported yet",
	}, {
		name: "mid-transition",
		subject: func() preflight {
			subject := ready()
			subject.Legacy = legacyState{Mode: "Direct", TargetMode: "Proxy"}
			return subject
		}(),
		check:    CheckMode,
		blocking: true,
		detail:   "a transition is in flight: Direct to Proxy",
	}, {
		// Work to do, not a reason the migration is impossible: pre-staging is something the
		// operator completes, and only they know whether it is done.
		name: "the syncer image is not on the nodes",
		subject: func() preflight {
			subject := ready()
			subject.RequiredImages["registrySyncer"] = "sha256:notheld"
			return subject
		}(),
		check:    CheckImagesPreStaged,
		blocking: false,
		detail:   "not resident on the nodes yet: the syncer",
	}, {
		name: "an operator wrote registry configuration for the nodes",
		subject: func() preflight {
			subject := ready()
			subject.NodeConfigurations = []nodeConfiguration{
				{Name: "extra-registry", TouchesContainerd: true},
				{Name: "sysctl-tuning"},
			}
			return subject
		}(),
		check:    CheckNodeConfiguration,
		blocking: false,
		detail:   "extra-registry",
	}}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			check := found(t, one.subject.report(), one.check)

			assert.False(t, check.Passed)
			assert.Equal(t, one.blocking, check.Blocking)
			assert.Contains(t, check.Detail, one.detail)
		})
	}
}

// Pre-staging is answered about the nodes, not about the DaemonSet's existence: images resident on
// some nodes and not others is exactly the case where the switch starts on a node that has to pull
// through the path being replaced.
func TestImagesAreOnlyPreStagedWhenEveryNodeHasThem(t *testing.T) {
	cases := []struct {
		name   string
		holder *imageHolderState
		detail string
	}{{
		name:   "no image holder at all",
		holder: nil,
		detail: "not keeping any images resident",
	}, {
		name:   "the holder is not on every node yet",
		holder: &imageHolderState{Images: []string{"x@sha256:aaa"}, Desired: 3, Available: 2},
		detail: "resident on 2 of 3 nodes",
	}, {
		// A DaemonSet scheduled nowhere holds nothing, however healthy it reads.
		name:   "the holder is scheduled nowhere",
		holder: &imageHolderState{Desired: 0, Available: 0},
		detail: "resident on 0 of 0 nodes",
	}}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			subject := ready()
			subject.ImageHolder = one.holder

			check := found(t, subject.report(), CheckImagesPreStaged)

			assert.False(t, check.Passed)
			assert.Contains(t, check.Detail, one.detail)
		})
	}
}

// A digest nobody supplied cannot be reported as absent from the nodes. The values are missing on
// a cluster whose module has not been rendered with image digests, and claiming the image is not
// pre-staged there would be a guess that reads exactly like a finding.
func TestAnImageWithNoKnownDigestIsNotReportedAsMissing(t *testing.T) {
	subject := ready()
	subject.RequiredImages = map[string]string{"dockerDistribution": "sha256:aaa"}

	check := found(t, subject.report(), CheckImagesPreStaged)

	assert.True(t, check.Passed, check.Detail)
}

// Unreadable node configurations are reported as unread, not as "none".
func TestNodeConfigurationsThatCannotBeReadAreNotReportedAsAbsent(t *testing.T) {
	subject := ready()
	subject.NodeConfigurationsUnreadable = errors.New("converting the snapshot value")

	check := found(t, subject.report(), CheckNodeConfiguration)

	assert.False(t, check.Passed)
	assert.Contains(t, check.Detail, "could not be read")
}

// Nothing is reported where there is no decision to make: a cluster installed on this
// implementation has no previous state, and a migrated one has the handover behind it. The second
// is the one that bites — the previous implementation's state secret outlives the migration, so
// every check would go on being answerable, and an unreachable-registry answer would tell an
// operator not to start a migration that finished months ago.
func TestNothingIsReportedWhereThereIsNoDecisionToMake(t *testing.T) {
	migrated := ready()
	migrated.AlreadySwitched = true
	assert.Empty(t, migrated.report(), "a migrated cluster is still asked about its migration")

	// Blocking answers in particular: this is what would reach an operator as an alert.
	migrated.CheckerStatus = checker.Status{Ready: false, Message: "dial tcp: i/o timeout"}
	assert.Empty(t, migrated.report())

	installedOnThisOne := ready()
	installedOnThisOne.HasLegacy = false
	installedOnThisOne.Legacy = legacyState{}
	assert.Empty(t, installedOnThisOne.report(),
		"a cluster that never ran the previous implementation is asked about a migration")
}

// The DaemonSet filter answers two questions: which images are kept resident, and on how many
// nodes. Both come from a real object, because the container names carrying the images are a
// convention of the previous implementation's template and nothing else enforces it.
func TestTheImageHolderFilterReadsTheHeldImagesAndTheNodeCount(t *testing.T) {
	daemonSet := &v1apps.DaemonSet{
		TypeMeta:   v1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"},
		ObjectMeta: v1.ObjectMeta{Name: ImageHolderName, Namespace: "d8-system"},
		Spec: v1apps.DaemonSetSpec{
			Template: v1core.PodTemplateSpec{
				Spec: v1core.PodSpec{
					Containers: []v1core.Container{
						{Name: "nodeservices-manager", Image: "registry.example.com/manager@sha256:mgr"},
						{Name: "image-holder-distribution", Image: "registry.example.com/x@sha256:aaa"},
						{Name: "image-holder-auth", Image: "registry.example.com/x@sha256:bbb"},
					},
				},
			},
		},
		Status: v1apps.DaemonSetStatus{DesiredNumberScheduled: 3, NumberAvailable: 3},
	}

	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(daemonSet)
	require.NoError(t, err)

	result, err := filterImageHolder(&unstructured.Unstructured{Object: object})
	require.NoError(t, err)

	state, ok := result.(imageHolderState)
	require.True(t, ok)

	// The manager runs in the same pod and is not an image being kept resident for somebody else.
	assert.Equal(t, []string{
		"registry.example.com/x@sha256:aaa",
		"registry.example.com/x@sha256:bbb",
	}, state.Images)
	assert.Equal(t, int32(3), state.Desired)
	assert.Equal(t, int32(3), state.Available)
}

// The node configuration filter recognises what a configuration writes, not what it is called: an
// operator names these anything, and the question is whether the transition will overwrite it.
func TestNodeConfigurationsAreRecognisedByWhatTheyWrite(t *testing.T) {
	cases := []struct {
		name    string
		content string
		touches bool
	}{{
		name:    "containerd v1 registry configuration",
		content: "mkdir -p /etc/containerd/conf.d\ncat > /etc/containerd/conf.d/mirror.toml <<EOF\nEOF",
		touches: true,
	}, {
		name:    "containerd v2 registry hosts",
		content: "bb-sync-file /etc/containerd/registry.d/example.com/hosts.toml -",
		touches: true,
	}, {
		name:    "registry mirrors in the runtime config",
		content: `[plugins."io.containerd.grpc.v1.cri".registry.mirrors]`,
		touches: true,
	}, {
		name:    "something unrelated to the registry",
		content: "sysctl -w net.ipv4.tcp_keepalive_time=120",
		touches: false,
	}, {
		name:    "no content at all",
		content: "",
		touches: false,
	}}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			object := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "deckhouse.io/v1alpha1",
				"kind":       "NodeGroupConfiguration",
				"metadata":   map[string]interface{}{"name": "written-by-an-operator"},
			}}
			if one.content != "" {
				require.NoError(t, unstructured.SetNestedField(
					object.Object, one.content, "spec", "content"))
			}

			result, err := filterNodeConfiguration(object)
			require.NoError(t, err)

			configuration, ok := result.(nodeConfiguration)
			require.True(t, ok)
			assert.Equal(t, "written-by-an-operator", configuration.Name)
			assert.Equal(t, one.touches, configuration.TouchesContainerd)
		})
	}
}
