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

const { mockDeep } = require('jest-mock-extended');
const waitForJobUnderTest = require('./validate-job-in-workflow-is-ready');

describe('GitHub Actions Workflow Checker', () => {
  let github;
  let context;
  let core;

  beforeEach(() => {
    github = mockDeep();
    context = {
      repo: {
        owner: 'test-owner',
        repo: 'test-repo'
      }
    };
    core = {
      info: jest.fn((str) => console.log(str)),
      setFailed: jest.fn((str) => console.error(str))
    };

    waitForJobInWorkflowIsCompletedWithSuccess = waitForJobUnderTest({ github, context, core });

    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  ({ github, context, core });

  describe('isJobInWorkflowOrWorkflowCompleted', () => {
    it('should return true when job is successfully completed', async () => {
      // Mock workflow runs response
      github.rest.actions.listWorkflowRunsForRepo.mockResolvedValue({
        data: {
          workflow_runs: [
            {
              id: 1,
              name: 'test-workflow',
              status: 'completed',
              conclusion: 'success',
              run_attempt: 1
            }
          ]
        }
      });

      const result = await waitForJobInWorkflowIsCompletedWithSuccess('main', 'test-workflow', 'test-job');

      console.log(result);
      expect(result).toBe(true);
      expect(core.setFailed).not.toHaveBeenCalled();
    });
  });

  describe('waitForJobInWorkflowIsCompletedWithSuccess', () => {
    it('should retry until job completes', async () => {
      let attempt = 0;

      github.rest.actions.listWorkflowRunsForRepo.mockImplementation(() => {
        attempt++;
        return {
          data: {
            workflow_runs: [
              {
                id: 1,
                name: 'test-workflow',
                status: attempt > 2 ? 'completed' : 'in_progress',
                run_attempt: 1,
                conclusion: 'success'
              }
            ]
          }
        };
      });

      github.rest.actions.listJobsForWorkflowRunAttempt.mockResolvedValue({
        data: {
          jobs: [
            {
              name: 'test-job',
              status: 'completed',
              conclusion: 'success'
            }
          ]
        }
      });

      const resultPromise = waitForJobInWorkflowIsCompletedWithSuccess('main', 'test-workflow', 'test-job', 5, 1000);

      await jest.advanceTimersByTimeAsync(5000);
      const result = await resultPromise;

      expect(result).toBe(true);
      expect(github.rest.actions.listWorkflowRunsForRepo).toHaveBeenCalledTimes(3);
    });

    it('should timeout when max attempts reached', async () => {
      github.rest.actions.listWorkflowRunsForRepo.mockResolvedValue({
        data: {
          workflow_runs: [
            {
              id: 1,
              name: 'test-workflow',
              status: 'in_progress',
              run_attempt: 1
            }
          ]
        }
      });

      github.rest.actions.listJobsForWorkflowRunAttempt.mockResolvedValue({
        data: {
          jobs: [
            {
              name: 'test-job',
              status: 'in_progress',
              conclusion: null
            }
          ]
        }
      });

      await expect(waitForJobInWorkflowIsCompletedWithSuccess('main', 'test-workflow', 'test-job', 3, 1000)).rejects.toThrow(
        'Max attempts reached'
      );

      expect(core.setFailed).toHaveBeenCalledWith('⌛ Timeout waiting for workflow completion');
    });
  });

  describe('Edge Cases', () => {
    it('should handle missing completed workflow', async () => {
      github.rest.actions.listWorkflowRunsForRepo.mockResolvedValue({
        data: {
          workflow_runs: [
            {
              id: 1,
              name: 'wrong-workflow',
              status: 'completed',
              run_attempt: 1
            }
          ]
        }
      });

      await waitForJobInWorkflowIsCompletedWithSuccess('main', 'test-workflow', 'test-job', 1, 1000);

      expect(core.setFailed).toHaveBeenCalledWith('❌ No completed workflow found');
    });

    it('should handle multiple active workflow runs', async () => {
      github.rest.actions.listWorkflowRunsForRepo.mockResolvedValue({
        data: {
          workflow_runs: [
            {
              id: 2,
              name: 'test-workflow',
              status: 'in_progress',
              run_attempt: 2
            },
            {
              id: 1,
              name: 'test-workflow',
              status: 'in_progress',
              run_attempt: 1
            }
          ]
        }
      });

      github.rest.actions.listJobsForWorkflowRunAttempt.mockResolvedValue({
        data: {
          jobs: [
            {
              name: 'test-job',
              status: 'completed',
              conclusion: 'success'
            }
          ]
        }
      });

      const result = await waitForJobInWorkflowIsCompletedWithSuccess('main', 'test-workflow', 'test-job', 1, 1000);

      expect(result).toBe(true);
      expect(github.rest.actions.listJobsForWorkflowRunAttempt).toHaveBeenCalledWith({
        owner: 'test-owner',
        repo: 'test-repo',
        run_id: 2,
        attempt_number: 2,
        per_page: 100
      });
    });
  });
});
