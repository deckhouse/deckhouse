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


/*
example use:
```javascript
const waitForWorkflowIsCompleted = require('./.github/scripts/js/validators/validate-workflow-is-ready')({ github, context, core });
try {
    await waitForWorkflowIsCompleted(branchName, 'Build and test for dev branches');
} catch(error) {
    core.setFailed(error);
}
```
*/

const WORKFLOW_STATUS_RUNNING = 'in_progress';
const WORKFLOW_STATUS_COMPLETED = 'completed';
const MAX_ATTEMPTS = 60;
const TIMEOUT_BETWEEN_ATTEMPT = 1000 * 30;
const MAX_ITEMS_PER_PAGE = 100;

module.exports = ({ github, context, core }) => {
  async function isWorkflowCompleted(branch, workflowName) {
    try {
      const { data } = await github.rest.actions.listWorkflowRunsForRepo({
        owner: context.repo.owner,
        repo: context.repo.repo,
        branch: branch,
        per_page: MAX_ITEMS_PER_PAGE,
      });

      const activeRuns = data.workflow_runs.filter(
        (run) => run.name === workflowName && run.status === WORKFLOW_STATUS_RUNNING
      );

      if (activeRuns.length > 0) {
        core.info(`🔄 Active '${workflowName}' workflows found, waiting...`);
        return false;
      }

      const completedRun = data.workflow_runs.find(
        (run) => run.name === workflowName && run.status === WORKFLOW_STATUS_COMPLETED
      );

      if (!completedRun) {
        core.setFailed('❌ No completed workflow found');
        return false;
      }

      return completedRun.conclusion === 'success';
    } catch (error) {
      core.setFailed(`🔥 Error: ${error.message}`);
      return false;
    }
  }

  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  return async function waitForWorkflowIsCompleted(
    branchName,
    workflowName,
    maxAttempts = MAX_ATTEMPTS,
    timeoutBetweenAttempt = TIMEOUT_BETWEEN_ATTEMPT
  ) {
    for (let i = 0; i < maxAttempts; i++) {
      core.info(`🚀 Attempt ${i + 1}/${maxAttempts}`);
      const isReady = await isWorkflowCompleted(branchName, workflowName);

      if (isReady) {
        core.info('✅ Workflow completed successfully!');
        return true;
      }

      await sleep(timeoutBetweenAttempt);
    }

    core.setFailed('⌛ Timeout waiting for workflow completion');
    throw new Error('Max attempts reached');
  };
};
