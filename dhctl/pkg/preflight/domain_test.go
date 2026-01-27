// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestChecker_CheckLocalhostDomain(t *testing.T) {
	tests := []struct {
		name          string
		skipFlag      bool
		executeError  error
		executeOutput []byte
		expectedError string
		setupMock     func(*mockNodeInterface, *mockScript)
	}{
		{
			name:     "skip flag enabled",
			skipFlag: true,
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				// No mock setup needed as function should return early
			},
		},
		{
			name:          "successful localhost resolution",
			skipFlag:      false,
			executeError:  nil,
			executeOutput: []byte("localhost resolves correctly\n"),
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte("localhost resolves correctly\n"), nil)
			},
		},
		{
			name:          "localhost resolution failed with exit error",
			skipFlag:      false,
			executeError:  &exec.ExitError{Stderr: []byte("localhost resolution failed")},
			executeOutput: []byte("Resolution failed"),
			expectedError: "Localhost domain resolving check failed:",
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				exitErr := &exec.ExitError{Stderr: []byte("localhost resolution failed")}
				msc.On("Execute", mock.Anything).Return([]byte("Resolution failed"), exitErr)
			},
		},
		{
			name:          "generic execution error",
			skipFlag:      false,
			executeError:  errors.New("network error"),
			executeOutput: []byte(""),
			expectedError: "Could not execute a script to check for localhost domain resolution:",
			setupMock: func(mni *mockNodeInterface, msc *mockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte(""), errors.New("network error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipFlag := app.PreflightSkipResolvingLocalhost
			defer func() {
				app.PreflightSkipResolvingLocalhost = originalSkipFlag
			}()

			app.PreflightSkipResolvingLocalhost = tt.skipFlag

			mockNode := &mockNodeInterface{}
			mockScript := &mockScript{}
			tt.setupMock(mockNode, mockScript)

			checker := &Checker{
				nodeInterface: mockNode,
			}

			err := checker.CheckLocalhostDomain(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockScript.AssertExpectations(t)
		})
	}
}

func TestChecker_CheckPublicDomainTemplate(t *testing.T) {
	tests := []struct {
		name          string
		skipFlag      bool
		metaConfig    *config.MetaConfig
		expectedError string
	}{
		{
			name:     "skip flag enabled",
			skipFlag: true,
		},
		{
			name:     "no global module config",
			skipFlag: false,
			metaConfig: &config.MetaConfig{
				ModuleConfigs: []*config.ModuleConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-module",
						},
					},
				},
				ClusterConfig: map[string]json.RawMessage{
					"clusterDomain": []byte(`"cluster.local"`),
				},
			},
		},
		{
			name:     "valid public domain template",
			skipFlag: false,
			metaConfig: &config.MetaConfig{
				ModuleConfigs: []*config.ModuleConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "global",
						},
						Spec: config.ModuleConfigSpec{
							Settings: map[string]interface{}{
								"modules": map[string]interface{}{
									"publicDomainTemplate": "example.com",
								},
							},
						},
					},
				},
				ClusterConfig: map[string]json.RawMessage{
					"clusterDomain": []byte(`"cluster.local"`),
				},
			},
		},
		{
			name:     "invalid public domain template - matches cluster domain",
			skipFlag: false,
			metaConfig: &config.MetaConfig{
				ModuleConfigs: []*config.ModuleConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "global",
						},
						Spec: config.ModuleConfigSpec{
							Settings: map[string]interface{}{
								"modules": map[string]interface{}{
									"publicDomainTemplate": "cluster.local",
								},
							},
						},
					},
				},
				ClusterConfig: map[string]json.RawMessage{
					"clusterDomain": []byte(`"cluster.local"`),
				},
			},
			expectedError: "The publicDomainTemplate \"cluster.local\" MUST NOT match the one specified in the clusterDomain parameter",
		},
		{
			name:     "invalid public domain template - contains cluster domain",
			skipFlag: false,
			metaConfig: &config.MetaConfig{
				ModuleConfigs: []*config.ModuleConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "global",
						},
						Spec: config.ModuleConfigSpec{
							Settings: map[string]interface{}{
								"modules": map[string]interface{}{
									"publicDomainTemplate": "test.cluster.local",
								},
							},
						},
					},
				},
				ClusterConfig: map[string]json.RawMessage{
					"clusterDomain": []byte(`"cluster.local"`),
				},
			},
			expectedError: "The publicDomainTemplate \"test.cluster.local\" MUST NOT match the one specified in the clusterDomain parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSkipFlag := app.PreflightSkipPublicDomainTemplateCheck
			defer func() {
				app.PreflightSkipPublicDomainTemplateCheck = originalSkipFlag
			}()

			app.PreflightSkipPublicDomainTemplateCheck = tt.skipFlag

			checker := &Checker{
				metaConfig: tt.metaConfig,
			}

			err := checker.CheckPublicDomainTemplate(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
