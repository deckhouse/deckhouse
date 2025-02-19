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

const TIMEOUT_BETWEEN_ATTEMPT = 1000 * 30;
const MAX_ATTEMPTS = 60;

module.exports = ({ github, context, core }) => {
  const { sleep } = require('../helpers/utils');
  const githubActions = require('../helpers/github-actions')({ github, context, core });

  /**
   *
   * @param {GithubWorkflow} workflowRun
   * @param {string} jobName
   *
   * @returns {GithubWorkflowJob}
   */
  async function getJobsForWorkflowRun(workflowRun, jobName) {
    return await githubActions.GetJobsForWorkflowRunAttempt(workflowRun.id, workflowRun.run_attempt, jobName);
  }

  /**
   *
   * @param {GithubWorkflow[]} workflowRuns
   * @param {string} jobName
   * @returns {boolean}
   */
  async function isJobInWorkflowCompletedSuccess(workflowRuns, jobName) {
    const workflowRun = workflowRuns[workflowRuns.length - 1];
    const jobs = await getJobsForWorkflowRun(workflowRun, jobName);
    if (jobs.length > 0) {
      const job = jobs[0];
      core.info(`Job status: ${job.status} | Job conclusion ${job.conclusion}`);
      return job.status === githubActions.WORKFLOW_STATUS_COMPLETED && job.conclusion === githubActions.CONCLUSION_STATUS_SUCCESS;
    }
    return false;
  }

  /**
   * @example await isJobInWorkflowOrWorkflowCompleted(branch, workflow_name, job_name);
   *
   * @param {string} branchName
   * @param {string} workflowName
   * @param {string} jobName
   *
   * @returns
   */
  async function isJobInWorkflowCompleted(branch, workflowName, jobName) {
    try {
      const activeRuns = await githubActions.GetWorkflowsByNameAndStatus(
        workflowName,
        branch,
        githubActions.WORKFLOW_STATUS_RUNNING
      );
      if (activeRuns.length > 0) {
        return isJobInWorkflowCompletedSuccess(activeRuns, jobName);
      }

      const completedRuns = await githubActions.GetWorkflowsByNameAndStatus(
        workflowName,
        branch,
        githubActions.WORKFLOW_STATUS_COMPLETED
      );
      if (completedRuns.length > 0) {
        return isJobInWorkflowCompletedSuccess(completedRuns, jobName);
      }

      return false;
    } catch (error) {
      console.error(`🔥 Error: ${error.message}`);
      return false;
    }
  }

  /**
   * @example await waitForJobInWorkflowIsCompletedWithSuccess(branch, workflow_name, job_name, 100);
   *
   * @param {string} branchName
   * @param {string} workflowName
   * @param {string} jobName
   * @param {int=} [maxAttempts=MAX_ATTEMPTS]
   * @param {int=} [timeoutBetweenAttempt=TIMEOUT_BETWEEN_ATTEMPT]
   *
   *
   * @returns
   */
  async function waitForJobInWorkflowIsCompletedWithSuccess(
    branchName,
    workflowName,
    jobName,
    maxAttempts = MAX_ATTEMPTS,
    timeoutBetweenAttempt = TIMEOUT_BETWEEN_ATTEMPT
  ) {
    for (let i = 0; i < maxAttempts; i++) {
      core.info(`🚀 Attempt ${i + 1}/${maxAttempts}`);
      const isReady = await isJobInWorkflowCompleted(branchName, workflowName, jobName);

      if (isReady) {
        core.info('✅ Job In Workflow completed successfully!');
        return true;
      }

      await sleep(timeoutBetweenAttempt);
    }

    // core.setFailed('⌛ Timeout waiting for workflow completion');
    throw new Error('Max attempts reached');
  }

  return {
    waitForJobInWorkflowIsCompletedWithSuccess,
    isJobInWorkflowCompleted
  };
};
