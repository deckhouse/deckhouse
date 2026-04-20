// Copyright 2026 Flant JSC
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

package checks

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/mocks"
)

func TestCheckLocalhostDomain(t *testing.T) {
	tests := []struct {
		name          string
		executeError  error
		executeOutput []byte
		expectedError string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockScript)
	}{
		{
			name:          "successful localhost resolution",
			executeError:  nil,
			executeOutput: []byte("localhost resolves correctly\n"),
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte("localhost resolves correctly\n"), nil)
			},
		},
		{
			name:          "localhost resolution failed with exit error",
			executeError:  &exec.ExitError{Stderr: []byte("localhost resolution failed")},
			executeOutput: []byte("Resolution failed"),
			expectedError: "Localhost domain resolving check failed:",
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				exitErr := &exec.ExitError{Stderr: []byte("localhost resolution failed")}
				msc.On("Execute", mock.Anything).Return([]byte("Resolution failed"), exitErr)
			},
		},
		{
			name:          "generic execution error",
			executeError:  errors.New("network error"),
			executeOutput: []byte(""),
			expectedError: "Could not execute a script to check for localhost domain resolution:",
			setupMock: func(mni *mocks.MockNodeInterface, msc *mocks.MockScript) {
				mni.On("UploadScript", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(msc)
				msc.On("Execute", mock.Anything).Return([]byte(""), errors.New("network error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockScript := &mocks.MockScript{}
			tt.setupMock(mockNode, mockScript)

			check := LocalhostDomainCheck{Node: mockNode}
			err := check.Run(context.Background())

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

func TestCheckPublicDomainTemplate(t *testing.T) {
	tests := []struct {
		name          string
		metaConfig    *config.MetaConfig
		expectedError string
	}{
		{
			name: "no global module config",
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
			name: "valid public domain template",
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
			name: "invalid public domain template - matches cluster domain",
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
			expectedError: "the publicDomainTemplate \"cluster.local\" must not match clusterDomain \"cluster.local\"",
		},
		{
			name: "invalid public domain template - contains cluster domain",
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
			expectedError: "the publicDomainTemplate \"test.cluster.local\" must not match clusterDomain \"cluster.local\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := PublicDomainTemplateCheck{MetaConfig: tt.metaConfig}
			err := check.Run(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
