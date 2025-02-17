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

const WORKFLOW_STATUS_RUNNING = 'in_progress';
const WORKFLOW_STATUS_COMPLETED = 'completed';
const CONCLUSION_STATUS_SUCCESS = 'success';
const MAX_ATTEMPTS = 60;
const TIMEOUT_BETWEEN_ATTEMPT = 1000 * 30;
const MAX_ITEMS_PER_PAGE = 100;

module.exports = ({ github, context, core }) => {
  /**
   *
   * @param {string} branch
   * @param {string} workflowName
   * @param {string} jobName
   * @returns
   */
  async function isJobInWorkflowOrWorkflowCompleted(branch, workflowName, jobName) {
    try {
      const { data } = await github.rest.actions.listWorkflowRunsForRepo({
        owner: context.repo.owner,
        repo: context.repo.repo,
        branch: branch,
        per_page: MAX_ITEMS_PER_PAGE
      });

      const activeRuns = data.workflow_runs.filter((run) => run.name === workflowName && run.status === WORKFLOW_STATUS_RUNNING);

      if (activeRuns.length > 0) {
        core.info(`🔄 Active '${workflowName}' workflows found...`);
        const activeRun = activeRuns[activeRuns.length - 1];
        core.info(`🔄 Last active '${workflowName}' workflow found, check job '${jobName}' status ...`);
        const { data } = await github.rest.actions.listJobsForWorkflowRunAttempt({
          owner: context.repo.owner,
          repo: context.repo.repo,
          run_id: activeRun.id,
          attempt_number: activeRun.run_attempt,
          per_page: MAX_ITEMS_PER_PAGE
        });

        // search job by jobName
        const jobs = data.jobs.filter((job) => job.name === jobName);
        const foundJob = jobs[0];

        core.info(foundJob.status, foundJob.conclusion);
        return foundJob.status === WORKFLOW_STATUS_COMPLETED && foundJob.conclusion === CONCLUSION_STATUS_SUCCESS;
      }
      return true;
    } catch (error) {
      core.setFailed(`🔥 Error: ${error.message}`);
      return false;
    }
  }

  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  /**
   *
   * @param {string} branchName
   * @param {string} workflowName
   * @param {string} jobName
   * @param {int=} [maxAttempts=MAX_ATTEMPTS]
   * @param {int=} [timeoutBetweenAttempt=TIMEOUT_BETWEEN_ATTEMPT]
   * @returns
   */
  return async function waitForJobInWorkflowIsCompletedWithSuccess(
    branchName,
    workflowName,
    jobName,
    maxAttempts = MAX_ATTEMPTS,
    timeoutBetweenAttempt = TIMEOUT_BETWEEN_ATTEMPT
  ) {
    for (let i = 0; i < maxAttempts; i++) {
      core.info(`🚀 Attempt ${i + 1}/${maxAttempts}`);
      const isReady = await isJobInWorkflowOrWorkflowCompleted(branchName, workflowName, jobName);

      if (isReady) {
        core.info('✅ Job In Workflow completed successfully!');
        return true;
      }

      await sleep(timeoutBetweenAttempt);
    }

    core.setFailed('⌛ Timeout waiting for workflow completion');
    throw new Error('Max attempts reached');
  };
};
