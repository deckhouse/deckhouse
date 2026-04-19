// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Mocks
const mockGithub = {
  rest: {
    actions: {
      listWorkflowRunsForRepo: jest.fn()
    }
  }
};

const mockGithubActions = {
  GetJobsForWorkflowRunAttempt: jest.fn(),
  CONCLUSION_STATUS_FAILURE: 'failure',
  CONCLUSION_STATUS_CANCELLED: 'cancelled',
  WORKFLOW_STATUS_COMPLETED: 'completed',
  CONCLUSION_STATUS_SUCCESS: 'success',
  GetWorkflowsByNameAndStatus: jest.fn()
};
const mockContext = {
  repo: {
    owner: 'owner',
    repo: 'repo'
  }
};
const mockCore = {
  info: jest.fn(),
  setFailed: jest.fn()
};

jest.mock('../helpers/github-actions', () => {
  return jest.fn(() => mockGithubActions);
});

// Set up the module with mocked dependencies
const { isJobInWorkflowCompleted, waitForJobInWorkflowIsCompletedWithSuccess } = require('./validate-job-in-workflow-is-ready')({
  github: mockGithub,
  context: mockContext,
  core: mockCore
});

describe('Job Workflow Tests', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should complete job successfully', async () => {
    mockGithub.rest.actions.listWorkflowRunsForRepo.mockResolvedValue({
      data: { workflow_runs: [{ name: 'workflow', status: mockGithubActions.WORKFLOW_STATUS_COMPLETED }] }
    });

    mockGithubActions.GetWorkflowsByNameAndStatus.mockResolvedValueOnce([{ id: 1 }]);
    mockGithubActions.GetJobsForWorkflowRunAttempt.mockResolvedValueOnce([{ status: 'completed', conclusion: 'success' }]);

    const result = await isJobInWorkflowCompleted('branch', 'workflow', 'job');
    console.log(result);
    expect(result).toEqual({ isReady: true, hasFailed: false });
  });

  it('should fail job due to cancellation', async () => {
    mockGithubActions.GetWorkflowsByNameAndStatus.mockResolvedValueOnce([{ id: 1 }]);
    mockGithubActions.GetJobsForWorkflowRunAttempt.mockResolvedValueOnce([{ status: 'completed', conclusion: 'cancelled' }]);

    const result = await isJobInWorkflowCompleted('branch', 'workflow', 'job');
    expect(result).toEqual({ isReady: false, hasFailed: true });
  });

  it('should return false when no workflows are active or completed', async () => {
    mockGithubActions.GetWorkflowsByNameAndStatus.mockResolvedValueOnce([]);

    const result = await isJobInWorkflowCompleted('branch', 'workflow', 'job');
    expect(result).toEqual({ isReady: false, hasFailed: false });
  });
});
