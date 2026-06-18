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

package probe

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func Test_initVirtualizationTimeouts(t *testing.T) {
	configs := initVirtualization(
		kubernetes.FakeAccessor(),
		checker.NoopDoer{},
		VirtualizationProbeConfig{VirtualImageURL: "https://example.com/alpine.qcow2"},
		logrus.New(),
	)

	assert.Len(t, configs, 2)

	creation := configs[0]
	assert.Equal(t, checker.VirtualizationCreationProbeName, creation.probe)
	assert.Equal(t, 5*time.Minute, creation.period)
	creationConfig := creation.config.(checker.VirtualMachineLifecycle)
	assert.False(t, creationConfig.VerifyLifecycle)
	assert.Equal(t, 4*time.Minute+30*time.Second, creationConfig.Timeout)
	assert.Equal(t, 60*time.Second, creationConfig.WaitVirtualDiskTimeout)
	assert.Equal(t, 30*time.Second, creationConfig.WaitVirtualMachineTimeout)

	lifecycle := configs[1]
	assert.Equal(t, checker.VirtualizationLifecycleProbeName, lifecycle.probe)
	assert.Equal(t, 15*time.Minute, lifecycle.period)
	lifecycleConfig := lifecycle.config.(checker.VirtualMachineLifecycle)
	assert.True(t, lifecycleConfig.VerifyLifecycle)
	assert.Equal(t, 10*time.Minute, lifecycleConfig.Timeout)
	assert.Equal(t, 2*time.Minute, lifecycleConfig.WaitVirtualDiskTimeout)
	assert.Equal(t, time.Minute, lifecycleConfig.WaitVirtualMachineTimeout)
	assert.Equal(t, 2*time.Minute, lifecycleConfig.WaitVirtualMachineMigrationTimeout)
}
