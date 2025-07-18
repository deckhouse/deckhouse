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

/**
 * @typedef JobStatusResult
 * @property {boolean} isReady
 * @property {boolean} hasFailed
 */

const TIMEOUT_BETWEEN_ATTEMPT = 1000 * 30;
const MAX_ATTEMPTS = 100;

module.exports = ({ github, context, core }) => {
  const { sleep } = require('../helpers/utils');
  const githubActions = require('../helpers/github-actions')({ github, context, core });

  /**
   * @param {GithubWorkflow} workflowRun
   * @param {string} jobName
   * @returns {GithubWorkflowJob}
   */
  async function getJobsForWorkflowRun(workflowRun, jobName) {
    return await githubActions.GetJobsForWorkflowRunAttempt(workflowRun.id, workflowRun.run_attempt, jobName);
  }

  /**
   * @param {GithubWorkflow[]} workflowRuns
   * @param {string} jobName
   * @returns {JobStatusResult}
   */
  async function isJobInWorkflowCompletedSuccess(workflowRuns, jobName) {
    const workflowRun = workflowRuns[0];
    const jobs = await getJobsForWorkflowRun(workflowRun, jobName);
    if (jobs.length === 0) {
      return { success: false, hasFailed: false };
    }

    const job = jobs[0];
    core.info(`Job status: ${job.status} | Job conclusion ${job.conclusion}`);
    core.info(`Job url: ${job.html_url}`);

    if (
      job.conclusion === githubActions.CONCLUSION_STATUS_FAILURE ||
      job.conclusion === githubActions.CONCLUSION_STATUS_CANCELLED
    ) {
      return { success: false, hasFailed: true };
    }

    const isSuccess =
      job.status === githubActions.WORKFLOW_STATUS_COMPLETED && job.conclusion === githubActions.CONCLUSION_STATUS_SUCCESS;
    return { success: isSuccess, hasFailed: false };
  }

  /**
   *
   * @param {string} branchName
   * @param {string} workflowName
   * @param {string} jobName
   * @returns {Promise<JobStatusResult>}
   */
  async function isJobInWorkflowCompleted(branchName, workflowName, jobName) {
    const activeRuns = await githubActions.GetWorkflowsByNameAndStatus(
      workflowName,
      branchName,
      githubActions.WORKFLOW_STATUS_RUNNING
    );
    if (activeRuns && activeRuns.length > 0) {
      const result = await isJobInWorkflowCompletedSuccess(activeRuns, jobName);
      if (result.hasFailed) return { isReady: false, hasFailed: true };
      return { isReady: result.success, hasFailed: false };
    }

    const completedRuns = await githubActions.GetWorkflowsByNameAndStatus(
      workflowName,
      branchName,
      githubActions.WORKFLOW_STATUS_COMPLETED
    );

    if (completedRuns && completedRuns.length > 0) {
      const result = await isJobInWorkflowCompletedSuccess(completedRuns, jobName);
      if (result.hasFailed) return { isReady: false, hasFailed: true };
      return { isReady: result.success, hasFailed: false };
    }

    return { isReady: false, hasFailed: false };
  }

  /**
   * @param {string} branchName
   * @param {string} workflowName
   * @param {string} jobName
   * @param {number} [maxAttempts=MAX_ATTEMPTS]
   * @param {number} [timeoutBetweenAttempt=TIMEOUT_BETWEEN_ATTEMPT]
   * @returns {Promise<boolean>}
   */
  async function waitForJobInWorkflowIsCompletedWithSuccess(
    branchName,
    workflowName,
    jobName,
    maxAttempts = MAX_ATTEMPTS,
    timeoutBetweenAttempt = TIMEOUT_BETWEEN_ATTEMPT
  ) {
    core.info(
      'wait for some time to exclude the moment when the task with the build has just been created and for some reason it is not visible'
    );
    await sleep(TIMEOUT_BETWEEN_ATTEMPT);

    for (let i = 0; i < maxAttempts; i++) {
      core.info(`🚀 Attempt ${i + 1}/${maxAttempts}`);
      const result = await isJobInWorkflowCompleted(branchName, workflowName, jobName);

      if (result.hasFailed) {
        core.setFailed('❌ Job failed or was cancelled');
        return false;
      }

      if (result.isReady) {
        core.info('✅ Job completed successfully!');
        return true;
      }

      await sleep(timeoutBetweenAttempt);
    }

    core.setFailed('⌛ Timeout waiting for workflow completion');
    return false;
  }

  return {
    waitForJobInWorkflowIsCompletedWithSuccess,
    isJobInWorkflowCompleted
  };
};
