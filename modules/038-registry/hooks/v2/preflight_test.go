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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
		Legacy:          legacyState{Mode: "Unmanaged"},
		CheckerReported: true,
		CheckerStatus:   checker.Status{Ready: true},
	}
}

func TestAReadyClusterPassesEveryCheck(t *testing.T) {
	checks := ready().report()

	require.Len(t, checks, 4, "every check has to be reported, including the ones that pass")
	for _, check := range checks {
		assert.Truef(t, check.Passed, "%s failed: %s", check.Name, check.Detail)
	}
}

// Which checks block and which only warn. Blocking says the migration must not begin at all, so
// which checks carry it is part of the contract rather than an implementation detail — and the two
// that only warn are the ones an operator, not this code, decides are done.
func TestWhichChecksBlockAndWhichOnlyWarn(t *testing.T) {
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
		// The stop that only exists in this build: none of the previous implementation's objects
		// render here, so a cluster arriving in Proxy has had the pull path they served deleted
		// from under it. "Settled" is not the same as "servable".
		name: "arrived still in Proxy",
		subject: func() preflight {
			subject := ready()
			subject.Legacy = legacyState{Mode: "Proxy"}
			return subject
		}(),
		check:    CheckMode,
		blocking: true,
		detail:   "return the previous release, bring it to Unmanaged there",
	}, {
		name: "arrived still in Direct",
		subject: func() preflight {
			subject := ready()
			subject.Legacy = legacyState{Mode: "Direct"}
			return subject
		}(),
		check:    CheckMode,
		blocking: true,
		detail:   "the cluster arrived in Direct",
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
			subject.Legacy = legacyState{Mode: "Unmanaged", TargetMode: "Proxy"}
			return subject
		}(),
		check:    CheckMode,
		blocking: true,
		detail:   "a transition is in flight: Unmanaged to Proxy",
	}, {
		// The one check that only warns: carrying user configuration over is work an operator
		// does and only they know is done.
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
