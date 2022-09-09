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
 * @param {string} inputs.state - A state type as 'success' (it is mark in github ui)
 * @param {string} inputs.description - A description for commit status
 * @param {string|undefined} inputs.url - A target url for commit status (Details link in github ui)
 * @returns Promise<bool>
 */
async function sendCreateCommitStatus({github, context, core, state, description, url}) {
  const commit_sha = process.env.STATUS_TARGET_COMMIT;
  core.debug(`sendCreateCommitStatus target commit: ${commit_sha}`);

  for(let i = 0; i < 3; i++) {
    const response = await github.rest.repos.createCommitStatus({
      owner: context.repo.owner,
      repo: context.repo.repo,
      sha: commit_sha,
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
 * @returns Promise<bool>
 */
async function setWait ({github, context, core}) {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'pending',
    description: 'Waiting for run e2e test.'
  })
}

/**
 * Set `e2e was failed` status (failed) when e2e was failed
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @returns Promise<bool>
 */
async function setFail({github, context, core}){
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'failure',
    description: 'E2e test was failed.',
    url: workflowUrl({core, context}),
  })
}

/**
 * Set `e2e was passed` status (failed) when e2e was failed
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @returns Promise<bool>
 */
function setSuccess ({github, context, core}) {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'success',
    description: 'E2e test was passed.',
    url: workflowUrl({core, context}),
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
 * @returns Promise<bool>
 */
async function setSkip({github, context, core}){
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'success',
    description: 'E2e test was skipped',
  })
}

/**
 * Set commit status when commit was pushed.
 * Check label for skipping e2e test and e2e tests should skip set success status
 * Used in build-and-test_dev workflow
 * Use STATUS_TARGET_COMMIT env var as target commit sha
 * Use PR_LABELS env var as list of PR labels
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @returns Promise<void>
 */
async function setInitialStatus ({github, context, core}) {
  const labels = JSON.parse(process.env.PR_LABELS);
  core.debug(`Labels: ${labels ? JSON.stringify(labels.map((l) => l.name)) : 'no labels'}`);

  const shouldSkip = labels ? labels.some((l) => l.name === "skip/e2e") : false;
  core.debug(`Should skip e2e: ${shouldSkip}`);

  const statusSetFunc = (shouldSkip) ? setSkip : setWait;

  const done = await statusSetFunc({github, context, core});
  if (!done) {
    core.setFailed('e2e requirement status was not set.');
  }
}

module.exports = {
  setSuccess,
  setFail,
  setInitialStatus
}
