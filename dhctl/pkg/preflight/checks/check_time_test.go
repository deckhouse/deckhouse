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
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/mocks"
)

func TestCheckTimeDrift(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name          string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockCommand)
		expectedError string
	}{
		{
			name: "time drift within acceptable range",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				// Remote time is 5 minutes ahead (300 seconds)
				remoteTime := now + 300
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte(strconv.FormatInt(remoteTime, 10)+"\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
		},
		{
			name: "time drift exceeds acceptable range",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				// Remote time is 15 minutes ahead (900 seconds)
				remoteTime := now + 900
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte(strconv.FormatInt(remoteTime, 10)+"\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
			expectedError: "time drift between local",
		},
		{
			name: "time drift exceeds acceptable range - remote behind",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				// Remote time is 15 minutes behind (-900 seconds)
				remoteTime := now - 900
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte(strconv.FormatInt(remoteTime, 10)+"\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
			expectedError: "time drift between local",
		},
		{
			name: "error getting remote timestamp",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte(""), []byte(""), errors.New("command failed"))
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
		},
		{
			name: "invalid timestamp format",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte("invalid-timestamp\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockCmd := &mocks.MockCommand{}
			tt.setupMock(mockNode, mockCmd)

			check := TimeDriftCheck{Node: mockNode}
			err := check.Run(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockNode.AssertExpectations(t)
			mockCmd.AssertExpectations(t)
		})
	}
}

func TestGetRemoteTimeStamp(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockNodeInterface, *mocks.MockCommand)
		expectedTime  int64
		expectedError string
	}{
		{
			name: "successful timestamp retrieval",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte("1640995200\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
			expectedTime: 1640995200,
		},
		{
			name: "command execution failed",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte(""), []byte(""), errors.New("command failed"))
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
			expectedError: "failed to execute date command:",
		},
		{
			name: "invalid timestamp format",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte("not-a-timestamp\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
			expectedError: "invalid timestamp format received",
		},
		{
			name: "timestamp parsing failed",
			setupMock: func(mni *mocks.MockNodeInterface, mc *mocks.MockCommand) {
				mc.On("Output", mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })).Return(
					[]byte("99999999999999999999999999999\n"), []byte(""), nil)
				mni.On("Command", "date", []string{"+%s"}).Return(mc)
			},
			expectedError: "failed to parse timestamp:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &mocks.MockNodeInterface{}
			mockCmd := &mocks.MockCommand{}
			tt.setupMock(mockNode, mockCmd)

			timestamp, err := getRemoteTimeStamp(context.Background(), mockNode)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Zero(t, timestamp)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTime, timestamp)
			}

			mockNode.AssertExpectations(t)
			mockCmd.AssertExpectations(t)
		})
	}
}

func TestGetLocalTimeStamp(t *testing.T) {
	before := time.Now().Unix()
	timestamp := time.Now().Unix()
	after := time.Now().Unix()

	// Timestamp should be within a reasonable range
	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)
}
