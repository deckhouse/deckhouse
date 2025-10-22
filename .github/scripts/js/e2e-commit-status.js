// Copyright 2022 Flant JSC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const {
  sleep
} = require('./time');

/**
 * Build workflow run url for add it to commit status target url
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @returns string
 */
function workflowUrl({core, context}) {
  core.debug(`workflowUrl context: ${JSON.stringify(context)}`);
  const {serverUrl, repo, runId} = context;
  const repository = repo.repo;
  const owner = repo.owner;
  const url = `${serverUrl}/${owner}/${repository}/actions/runs/${runId}`;
  core.debug(`workflowUrl url: ${url}`);
  return url
}

/**
 * Wrap github.rest.repos.createCommitStatus. Returns true if status was set
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {object} inputs.status - A state object for send
 * @param {string} inputs.status.state - A state type as 'success' (it is mark in GitHub ui)
 * @param {string} inputs.status.description - A description for commit status
 * @param {string|undefined} inputs.status.url - A target url for commit status (Details link in GitHub ui)
 * @param {string} inputs.status.commitSha - A commit for set status
 * @returns Promise<bool>
 */
async function sendCreateCommitStatus({github, context, core, status}) {
  const {state, description, url, commitSha} = status
  core.debug(`sendCreateCommitStatus target commit: ${commitSha}`);

  for(let i = 0; i < 3; i++) {
    const response = await github.rest.repos.createCommitStatus({
      owner: context.repo.owner,
      repo: context.repo.repo,
      sha: commitSha,
      state: state,
      description: description,
      target_url: url,
      context: 'E2e test'
    });

    core.debug(`rest.repos.createCommitStatus response: ${JSON.stringify(response)}`);
    if (response.status === 201) {
      core.debug(`rest.repos.createCommitStatus response status is 201. Returns true`);
      return true;
    }

    // wait 3s for retry request
    await sleep(3000);
  }

  return false
}

/**
 * Set `waiting for start e2e` status (pending) Uses with push commit
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {string} inputs.commitSha - sha commit for set status
 * @returns Promise<bool>
 */
async function setWait ({github, context, core, commitSha}) {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    status: {
      commitSha,
      state: 'pending',
      description: 'Waiting for run e2e test'
    }
  })
}

/**
 * Set `e2e was failed` status (failed) when e2e was failed
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {string} inputs.commitSha - sha commit for set status
 * @returns Promise<bool>
 */
async function setFail({github, context, core, commitSha}){
  return sendCreateCommitStatus({
    github,
    context,
    core,
    status: {
      commitSha,
      state: 'failure',
      description: 'E2e test was failed',
      url: workflowUrl({core, context}),
    }
  })
}

/**
 * Set `e2e was passed` status (failed) when e2e was failed
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {string} inputs.commitSha - sha commit for set status
 * @returns Promise<bool>
 */
function setSuccess ({github, context, core, commitSha}) {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    status: {
      commitSha,
      state: 'success',
      description: 'E2e test was passed',
      url: workflowUrl({core, context}),
    }
  })
}

/**
 * Set `e2e was failed` status (success) when e2e was failed
 * Unfortunately we do not have 'skip' status and use 'success'
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {string} inputs.commitSha - sha commit for set status
 * @returns Promise<bool>
 */
async function setSkip({github, context, core, commitSha}){
  return sendCreateCommitStatus({
    github,
    context,
    core,
    status: {
      commitSha,
      state: 'success',
      description: 'E2e test was skipped',
    },
  })
}

/**
 * Set commit status when label set/unset.
 * Check label for skipping e2e test and e2e tests should skip set success status
 * If status was not set then fail job
 *
 * Used in build-and-test_dev workflow
 *
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {boolean} inputs.labeled - true - PR was labeled, false - unlabeled
 * @param {string} inputs.commitSha - sha commit for set status
 * @returns Promise<void>
 */
async function onLabeledForSkip({github, context, core, labeled, commitSha}) {
  const statusSetFunc = (labeled) ? setSkip : setWait;

  const done = await statusSetFunc({github, context, core, commitSha});
  if (!done) {
    core.setFailed('e2e requirement status was not set.');
  }
}

/**
 * Set commit status when commit was pushed.
 * Check label for skipping e2e test and e2e tests should skip set success status
 * If status was not set then fail job
 *
 * Used in e2e_run* workflow
 *
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * Use STATUS_TARGET_COMMIT env var as job status
 * Use  env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @returns Promise<void>
 */
async function setStatusAfterE2eRun({github, context, core}) {
  const jobStatus = process.env.JOB_STATUS;
  const commitSha = process.env.STATUS_TARGET_COMMIT;

  let setStateFunc = null;
  if (jobStatus === 'failure' || jobStatus === 'cancelled') {
    setStateFunc = setFail;
  } else if (jobStatus === 'success') {
    setStateFunc = await setSuccess;
  } else {
    core.setFailed(`e2e requirement status was not set. Job status ${jobStat}`)
    return
  }

  const success = setStateFunc({github, context, core, commitSha})
  if (!success) {
    core.setFailed(`e2e requirement status was not set. Job status ${jobStat}`)
  }
}

/**
 * Set commit status when commit was pushed.
 * Check label for skipping e2e test and e2e tests should skip set success status
 * If status was not set then fail job
 *
 * Used in build-and-test_dev workflow
 *
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * Use PR_LABELS env var as list of PR labels
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @returns Promise<void>
 */
async function setInitialStatus ({github, context, core}) {
  core.info(`Labels json: ${process.env.PR_LABELS}`);

  const labels = JSON.parse(process.env.PR_LABELS);
  const commitSha = process.env.STATUS_TARGET_COMMIT;

  core.debug(`Labels: ${labels ? JSON.stringify(labels.map((l) => l.name)) : 'no labels'}`);

  const shouldSkip = labels ? labels.some((l) => l.name === "skip/e2e") : false;
  core.debug(`Should skip e2e: ${shouldSkip}`);

  return onLabeledForSkip({github, context, core, labeled: shouldSkip, commitSha})
}

module.exports = {
  setStatusAfterE2eRun,
  setInitialStatus,
  onLabeledForSkip
}
