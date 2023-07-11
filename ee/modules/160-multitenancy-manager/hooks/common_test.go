/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks_test

import (
	"testing"

	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

type testProjectStatus struct {
	exists     bool
	name       string
	conditions string
	status     string
}

func checkProjectStatus(f *HookExecutionConfig, tc testProjectStatus) {
	pr := f.KubernetesGlobalResource("Project", tc.name)
	Expect(pr.Exists()).To(Equal(tc.exists))

	if tc.exists {
		Expect(pr.Field("status.conditions")).To(MatchJSON(tc.conditions))
		Expect(pr.Field("status.statusSummary")).To(MatchJSON(tc.status))
	}
}
