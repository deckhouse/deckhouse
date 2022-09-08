const {
  sleep
} = require('./time');

function workflowUrl({core, context}) {
  core.debug(`workflowUrl context: ${JSON.stringify(context)}`);
  const {serverUrl, repo, runId} = context;
  const repository = repo.repo;
  const url = `${serverUrl}/${repository}/actions/runs/${runId}}`;
  core.debug(`workflowUrl url: ${url}`);
  return url
}

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
      context: 'deckhouse/e2e-requirement'
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

module.exports.setWait = async ({github, context, core}) => {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'pending',
    description: 'Waiting for run e2e test.'
  })
};

module.exports.setFail = async ({github, context, core}) => {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'failure',
    description: 'E2e test was failed.',
    url: workflowUrl({core, context}),
  })
};

module.exports.setSuccess = async ({github, context, core}) => {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'success',
    description: 'E2e test was passed.',
    url: workflowUrl({core, context}),
  })
};
