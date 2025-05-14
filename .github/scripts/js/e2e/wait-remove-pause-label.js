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

const prNumber = process.env.PR_NUMBER;
const maxWaitTimeMinutes = 60;
const startTime = Date.now();

module.exports = ({ github, context, core }) => {
  async function checkLabel() {
    try {
      const { data: pullRequest } = await github.rest.pulls.get({
        owner: context.repo.owner,
        repo: context.repo.repo,
        pull_number: parseInt(prNumber),
      });

      const labels = pullRequest.labels.map((label) => label.name);

      if (!labels.includes("e2e/pause")) {
        core.info('Label "e2e/pause" has been removed. Continuing...');
        return true; // Label removed
      } else {
        return false; // Label still present
      }
    } catch (error) {
      core.info(`Failed to get PR information: ${error} Continuing...`);
      return false;
    }
  }

  return async function waitForLabelRemoval() {
    while (true) {
      const labelRemoved = await checkLabel();
      if (labelRemoved) {
        return; // Exit the loop
      }

      const elapsedTimeMinutes = (Date.now() - startTime) / (1000 * 60);
      if (elapsedTimeMinutes >= maxWaitTimeMinutes) {
        core.setFailed(
          `Timeout: Waited ${maxWaitTimeMinutes} minutes for 'e2e/pause' label to be removed.`
        );
        return; // Exit the loop and fail the job
      }

      core.info('Label "e2e/pause" still present.  Waiting 60 seconds...');
      await new Promise((resolve) => setTimeout(resolve, 60000)); // Wait 60 seconds
    }
  };
};
