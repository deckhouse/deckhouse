const {
  sleep
} = require('./time');

function workflowUrl({server_url, repository, run_id}) {
  return `${server_url}/${repository}/actions/runs/${run_id}}`
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
    url: workflowUrl(context),
  })
};

module.exports.setSuccess = async ({github, context, core}) => {
  return sendCreateCommitStatus({
    github,
    context,
    core,
    state: 'success',
    description: 'E2e test was passed.',
    url: workflowUrl(context),
  })
};
