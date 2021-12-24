//@ts-check
const { knownLabels, labelsSrv, knownProviders, knownChannels } = require('./constants');

/**
 * Update a comment in "release" issue when workflow is started.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.name - A workflow name.
 * @returns {Promise<*>}
 */
module.exports.updateCommentOnStart = async ({ github, context, core, name }) => {
  const repo_url = context.payload.repository.html_url;
  const run_id = context.runId;
  const github_ref = context.ref;
  const build_url = `${repo_url}/actions/runs/${run_id}`;

  const comment_id = context.payload.inputs.comment_id;
  const issue_id = context.payload.inputs.issue_id;

  console.log(`
        issue_id: ${issue_id}
        comment_id: ${comment_id}
        build_url: ${build_url}
        context: ${JSON.stringify(context)}
  `);

  // Get existing comment.
  let response = await github.rest.issues.getComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: comment_id
  });

  if (response.status != 200) {
    return core.setFailed(`comment is not accessible ${JSON.stringify(response)}`);
  }

  const newBody =
    response.data.body +
    `
  :fast_forward:\u00a0Workflow \`${name}\` for \`${github_ref}\` [started](${build_url}).
`;

  response = await github.rest.issues.updateComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: comment_id,
    body: newBody
  });

  if (response.status != 200) {
    return core.setFailed(`comment is not accessible ${JSON.stringify(response)}`);
  }

  console.log(`Issue comment updated: ${response.data.html_url}.`);
};

/**
 * Update a comment in "release" issue with status of job or workflow.
 *
 * "job,inline" updates comment with job status and a name without extra newlines.
 * "job" updates comment with job status and a name with extra newlines.
 * "workflow" updates comment with statuses of all jobs in needs context with extra newlines.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.statusSource - 'job,inline', 'job' or 'workflow'.
 * @param {string} inputs.name - A name to use in the comment.
 * @param {object} inputs.needsContext - The needs context contains outputs from all jobs that are defined as a dependency of the current job.
 * @param {object} inputs.jobContext - The job context contains information about the currently running job.
 * @returns {Promise<*>}
 */
module.exports.updateCommentOnFinish = async ({ github, context, core, statusSource, name, needsContext, jobContext }) => {
  // Get comment
  const comment_id = context.payload.inputs.comment_id;
  const response = await github.rest.issues.getComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: comment_id
  });
  if (response.status != 200) {
    console.log(`DEBUG getComment response: ${JSON.stringify(response)}`);
    return core.setFailed(`comment is not accessible ${JSON.stringify(response)}`);
  }
  const comment = response.data.body;

  // Final status is a passed job.status or a summary from 'needs' context for workflow.
  let finalStatus = '';
  let jobsComment = '';

  if (statusSource === 'job') {
    finalStatus = jobContext.status;
  }

  if (statusSource === 'workflow') {
    // TODO (future) This is the last job, it can compare actual comment with needs object and restore lost comments.
    console.log(`DEBUG Needs: ${JSON.stringify(needsContext)}`);

    finalStatus = 'cancelled';
    let successCount = 0;
    let failureCount = 0;
    for (const jobName in needsContext) {
      if (!needsContext.hasOwnProperty(jobName)) {
        continue;
      }
      if (needsContext[jobName].result === 'success') {
        successCount++;
      }
      if (needsContext[jobName].result === 'failure') {
        failureCount++;
      }
      // Info about not started jobs.
      if (needsContext[jobName].result === 'cancelled') {
        jobsComment += `:ballot_box_with_check:\u00a0${jobName} cancelled.\n`;
      }
      if (needsContext[jobName].result === 'skipped') {
        jobsComment += `:ballot_box_with_check:\u00a0${jobName} skipped.\n`;
      }
    }
    if (successCount > 0) {
      finalStatus = 'success';
    }
    if (failureCount > 0) {
      finalStatus = 'failure';
    }
    if (jobsComment !== '') {
      jobsComment = '\n' + jobsComment;
    }
  }

  console.log(`Status is ${finalStatus}`);

  // Update comment.
  let newBody = '';
  if (statusSource.endsWith('inline')) {
    let statusComment = `:white_check_mark: \`${name}\` success.`;
    if (finalStatus === 'failure') {
      statusComment = `:x: \`${name}\` failed.`;
    }
    if (finalStatus === 'cancelled') {
      statusComment = `:ballot_box_with_check: \`${name}\` cancelled.`;
    }
    newBody = `${comment}\n${statusComment}`;
  } else {
    let statusComment = `:green_circle:\u00a0\`${name}\` succeed.`;
    if (finalStatus === 'failure') {
      statusComment = `:red_circle:\u00a0\`${name}\` failed.`;
    }
    if (finalStatus === 'cancelled') {
      statusComment = `:white_circle:\u00a0\`${name}\` cancelled.`;
    }
    if (finalStatus === 'skipped') {
      statusComment = `:white_circle:\u00a0\`${name}\` skipped.`;
    }
    newBody = `${comment}${jobsComment}\n\n${statusComment}\n\n`;
  }

  const updateResponse = await github.rest.issues.updateComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: comment_id,
    body: newBody
  });

  if (updateResponse.status != 200) {
    console.log(`DEBUG updateComment response: ${JSON.stringify(updateResponse)}`);
    return core.setFailed(`comment is not accessible ${JSON.stringify(updateResponse)}`);
  }
};

/**
 * Check if label is present on PR or "release" issue and set 'shouldRun'
 * output to run or skip next jobs. Also, removes the label.
 *
 * Outputs:
 * - shouldRun - 'true'/'false' indicates label presence.
 * - labels - an array of labels on issue or PR.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.labelType - A label prefix: 'e2e' or 'deploy-web'.
 * @param {string} inputs.labelSubject - A last part of the label.
 * @param {function} inputs.onSuccess - A callback function to run on success.
 * @returns {Promise<void|*>}
 */
const checkLabel = async ({ github, context, core, labelType, labelSubject, onSuccess }) => {
  const shouldRunLabel = labelsSrv.findLabel({ labelType, labelSubject });
  if (shouldRunLabel === '') {
    core.setOutput('should_run', 'false');
    return console.log(`Ignore unknown label for type='${labelType}' subject='${labelSubject}'. Skip next jobs.`);
  }

  let labels = null;
  let issue_number = '';
  let isPR = false;

  // Workflow started via workflow_dispatch, get labels by issue_id.
  if (context.eventName === 'workflow_dispatch') {
    issue_number = context.payload.inputs.issue_number;
    const response = await github.rest.issues.get({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issue_number
    });
    if (response.status != 200) {
      return core.setFailed(`Cannot get issue by number ${issue_number}: ${JSON.stringify(response)}`);
    }

    labels = response.data.labels;
  }

  // Workflow started via workflow_dispatch, search pull_request and get labels.
  if (context.eventName === 'push') {
    isPR = true;
    const response = await github.rest.repos.listPullRequestsAssociatedWithCommit({
      owner: context.repo.owner,
      repo: context.repo.repo,
      commit_sha: context.sha
    });
    if (response.status != 200) {
      return core.setFailed(`Cannot list PRs for commit ${context.sha}: ${JSON.stringify(response)}`);
    }
    // Get first associated pr.
    if (response.data && response.data.length > 0) {
      const pr = response.data.length > 0 && response.data[0];
      labels = pr.labels;
      issue_number = pr.number;
    } else {
      // Return if no PR. Do not fail for 'push' event, as these jobs can be restarted later.
      return console.log(
        `Something bad happens. No issue or pull_request found. event_name=${context.eventName} action=${context.action} ref=${context.ref}`
      );
    }
  }

  console.log(
    `'${context.eventName}' event for ${isPR ? 'PR' : 'issue'} #${issue_number} with labels: ${JSON.stringify(
      labels.map((l) => l.name)
    )}`
  );
  core.setOutput('labels', JSON.stringify(labels));

  if (!labels) {
    return core.setFailed(
      `No issue or PR found or unknown event is occurred. event_name=${context.eventName} action=${context.action} ref=${context.ref}`
    );
  }

  let hasLabel = false;
  for (const label of labels) {
    if (label.name === shouldRunLabel) {
      hasLabel = true;
    }
  }

  core.setOutput('should_run', hasLabel.toString());

  if (onSuccess) {
    onSuccess({ labels, hasLabel });
  }

  if (!hasLabel) {
    console.log(`${isPR ? 'PR' : 'Issue'} #${issue_number} has no label '${shouldRunLabel}'. Skip next jobs.`);
    return;
  }
  // Remove label
  console.log(`Requested label '${shouldRunLabel}' is present. Remove it now...`);
  try {
    await github.rest.issues.removeLabel({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issue_number,
      name: shouldRunLabel
    });
    console.log(`  Done.`);
  } catch (e) {
    console.log(`  It seems label was removed by another workflow. Ignore ${typeof e} error.`);
  }
  console.log(`Proceed to next jobs.`);
};
module.exports.checkLabel = checkLabel;

/**
 * Check e2e/use labels to determine which cri/version job to run for provider.
 *
 * This method set 'true'/'false' outputs for each cri/version job.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.provider - A slug of the provider.
 * @param {object} inputs.defaults - CRI type and Kubernetes version to use if no e2e/use labels set.
 * @param {string[]} inputs.criNames - Names of cri types available for e2e tests.
 * @param {string[]} inputs.kubernetesVersions - Names of Kubernetes versions available for e2e tests.
 * @returns {Promise<void>}
 */
module.exports.checkE2ELabels = async ({ github, context, core, provider, defaults, criNames, kubernetesVersions }) => {
  // Get labels from PR
  let issueLabels = [];
  let shouldRun = false;

  await checkLabel({
    github,
    context,
    core,
    labelType: 'e2e',
    labelSubject: provider,
    onSuccess: ({ labels, hasLabel }) => {
      issueLabels = labels;
      shouldRun = hasLabel;
    }
  });

  if (!shouldRun) {
    console.log(`No e2e label for provider '${provider}'. Skip next jobs.`);
    return;
  }

  let useLabels = [];
  if (issueLabels) {
    for (const label of issueLabels) {
      if (label.name.startsWith('e2e/use')) {
        useLabels.push(label.name);
      }
    }
  }
  console.log(`e2e/use labels: ${JSON.stringify(useLabels)}`);

  if (useLabels.length === 0) {
    const cri = defaults.criName.toLowerCase();
    const ver = defaults.kubernetesVersion.replace(/\./g, '_'); // replaceAll is undefined in node js
    core.setOutput(`run_${cri}_${ver}`, 'true');
    return console.log(`No additional 'e2e/use/' labels found. Will run e2e with default cri=${cri} and version=${ver}.`);
  }

  let hasCriLabel = false;
  let hasVerLabel = false;
  for (const label of useLabels) {
    if (label.startsWith('e2e/use/cri')) {
      hasCriLabel = true;
    }
    if (label.startsWith('e2e/use/k8s')) {
      hasVerLabel = true;
    }
  }

  for (const criName of criNames) {
    for (const kubernetesVersion of kubernetesVersions) {
      const cri = criName.toLowerCase();
      const ver = kubernetesVersion.replace(/\./g, '_');

      let hasCri = false;
      let hasVer = false;
      for (const label of useLabels) {
        if (label === `e2e/use/cri/${cri}`) {
          hasCri = true;
          // Use default kubernetes version if there is no e2e/use/k8s label.
          if (!hasVerLabel && kubernetesVersion === defaults.kubernetesVersion) {
            hasVer = true;
          }
        }
        if (label === `e2e/use/k8s/${kubernetesVersion}`) {
          hasVer = true;
          // Use default CRI if there is no e2e/use/cri label.
          if (!hasCriLabel && criName === defaults.criName) {
            hasCri = true;
          }
        }
      }

      const shouldRun = hasCri && hasVer ? 'true' : 'false';
      core.setOutput(`run_${cri}_${ver}`, shouldRun);
      console.log(`run_${cri}_${ver}: ${hasCri} && ${hasVer} == ${shouldRun}`);
    }
  }
};

/**
 * Check 'skip validation' labels, set boolean outputs for validation jobs.
 *
 * Outputs:
 * - run_<validation_type> - A boolean to start or skip a job.
 * - label_<validation_type> - A label name to use in failure message.
 * - diff_url - An URL to fetch full diff for PR.
 * - pr_title - A title of PR.
 * - pr_description - A description of PR.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void|*>}
 */
module.exports.checkValidationLabels = async ({ github, context, core }) => {
  // Get first associated PR.
  const response = await github.rest.repos.listPullRequestsAssociatedWithCommit({
    owner: context.repo.owner,
    repo: context.repo.repo,
    commit_sha: context.sha
  });
  if (response.status != 200) {
    return core.setFailed(`Cannot list PRs for commit ${context.sha}: ${JSON.stringify(response)}`);
  }

  // Run all validations by default.
  core.setOutput('run_no_cyrillic', 'true');
  core.setOutput('run_doc_changes', 'true');
  core.setOutput('run_copyright', 'true');

  // No PR found. It can be first push before creating PR.
  // Assume there are no labels and all validations should run
  // using commit diff as input for validation scripts.
  //
  // context.payload.compare contains a proper url
  // for one commit and for multiple commits in branch.
  if (!response.data || response.data.length === 0) {
    const diff_url = context.payload.compare + '.diff';
    core.setOutput('diff_url', diff_url);
    console.log(`diff_url="${diff_url}"`);

    return console.log(
      `No pull_request found. Run all validations. event_name=${context.eventName} action=${context.action} ref=${context.ref}`
    );
  }

  const pr = response.data[0];

  // Check labels and disable corresponding validations.
  for (const skipLabel of knownLabels['skip-validation']) {
    let prHasSkipLabel = pr.labels.some((l) => l.name === skipLabel);

    let validationName = '';
    if (/no-cyrillic/.test(skipLabel)) {
      validationName = 'no_cyrillic';
      console.log(`Skip 'no-cyrillic'`);
    }
    if (/documentation/.test(skipLabel)) {
      validationName = 'doc_changes';
      console.log(`Skip 'doc-changes'`);
    }
    if (/copyright/.test(skipLabel)) {
      validationName = 'copyright';
      console.log(`Skip 'copyright'`);
    }

    if (prHasSkipLabel) {
      core.setOutput(`run_${validationName}`, 'false');
    }
    core.setOutput(`label_${validationName}`, skipLabel);
  }

  core.setOutput('pr_title', pr.title);
  console.log(`pr_title='${pr.title}'`);

  core.setOutput('pr_description', pr.body);
  console.log(`pr_description='${pr.body}'`);

  core.setOutput('diff_url', pr.diff_url);
  console.log(`diff_url='${pr.diff_url}'`);
};

/**
 * Get all labels from release issue and determine a workflow to run next.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void|*>}
 */
module.exports.runWorkflowForReleaseIssue = async ({ github, context, core }) => {
  const event = context.payload;
  const label = event.label.name;
  const lowerLabel = label.toLowerCase();

  console.log(`Event label name: ${label}`);
  console.log(`Known labels: ${JSON.stringify(knownLabels, null, '  ')}`);

  let workflow_id = '';

  if (knownLabels.e2e.includes(label)) {
    for (const provider of knownProviders) {
      if (label.includes(provider)) {
        workflow_id = `e2e-${provider}.yml`;
        break;
      }
    }
  }

  if (knownLabels['deploy-web'].includes(label)) {
    for (const webEnv of ['test', 'stage']) {
      if (label.includes(webEnv)) {
        workflow_id = `deploy-web-${webEnv}.yml`;
        break;
      }
    }
  }

  let isDeployChannel = false;
  if (knownLabels.deploy.includes(label)) {
    for (const channel of knownChannels) {
      if (lowerLabel.includes(channel)) {
        workflow_id = `deploy-${channel}.yml`;
        isDeployChannel = true;
        break;
      }
    }
  }

  // suspend release in a channel
  if (knownLabels.suspend.includes(label)) {
    for (const channel of knownChannels) {
      if (lowerLabel.includes(channel)) {
        workflow_id = `suspend-${channel}.yml`;
        isDeployChannel = true;
        break;
      }
    }
  }

  if (knownLabels['skip-validation'].includes(label)) {
    workflow_id = 'validation.yml';
  }

  if (workflow_id === '') {
    return console.log(`Workflow for label "${event.label.name}" not found. Ignore it.`);
  }

  let hasProperLabel = false;
  for (const label of event.issue.labels) {
    if (label.name === knownLabels['issue-release']) {
      hasProperLabel = true;
    }
  }
  if (!hasProperLabel) {
    return core.setFailed(`Issue #${event.issue.number} requires label 'issue/release' to run workflow for label '${label}'.`);
  }

  // Calculate ref for workflow:
  // - search tag by issue.milestone.title
  // - use refs/heads/main if no tag
  // - use refs/tags/TAG if tag is found.
  console.log(`Search for tag ${event.issue.milestone.title}`);
  let ref = 'refs/heads/main';
  try {
    const response = await github.rest.git.getRef({
      owner: context.repo.owner,
      repo: context.repo.repo,
      ref: `tags/${event.issue.milestone.title}`
    });
    if (response && response.status == 200) {
      ref = `refs/tags/${event.issue.milestone.title}`;
    }
    console.log(JSON.stringify(response));
  } catch (error) {
    console.log(`get tag error: ${JSON.stringify(error)}`);
  }

  console.log(`Use ref=${ref}`);

  // Return if workflow is deploy-channel but no tag is pushed.
  if (!ref.startsWith('refs/tags/') && isDeployChannel) {
    return core.setFailed(`Workflow for label ${label} requires a tag. ${event.issue.milestone.title} is not found.`);
  }

  // Add issue comment.
  console.log('Add issue comment.');
  let response = await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: event.issue.number,
    body: `Run workflow "${event.label.name}"...`
  });

  if (response.status < 200 || response.status >= 300) {
    return core.setFailed(`Cannot start workflow: ${JSON.stringify(response)}`);
  }

  console.log(`Start workflow '${workflow_id}' with ref '${ref}'.`);
  const issue_id = '' + event.issue.id;
  const issue_number = '' + event.issue.number;
  const comment_id = '' + response.data.id;
  response = await github.rest.actions.createWorkflowDispatch({
    owner: context.repo.owner,
    repo: context.repo.repo,
    workflow_id: workflow_id,
    ref: ref,
    inputs: { issue_id, issue_number, comment_id }
  });

  if (response.status > 200 && response.status < 300) {
    console.log('Workflow started successfully');
  } else {
    return core.setFailed(`Error calling dispatch. Response: ${JSON.stringify(response)}`);
  }
};

/**
 * Get labels from PR and determine a workflow to run next.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void>}
 */
module.exports.runWorkflowForPullRequest = async ({ github, context, core }) => {
  const event = context.payload;
  const label = event.label.name;
  console.log(`Event label name: '${label}'`);
  console.log(`Known labels: ${JSON.stringify(knownLabels, null, '  ')}`);

  let workflow_id = '';

  if (knownLabels.e2e.includes(label) && event.action === 'labeled') {
    for (const provider of knownProviders) {
      if (label.includes(provider)) {
        workflow_id = `e2e-${provider}.yml`;
        break;
      }
    }
  }

  if (knownLabels['deploy-web'].includes(label) && event.action === 'labeled') {
    // prod env is not available for pull request.
    for (const webEnv of ['test', 'stage']) {
      if (label.includes(webEnv)) {
        workflow_id = `deploy-web-${webEnv}.yml`;
        break;
      }
    }
  }

  if (knownLabels['skip-validation'].includes(label)) {
    workflow_id = 'validation.yml';
  }

  if (workflow_id === '') {
    return console.log(`Workflow for label '${event.label.name}' and action '${event.action}' not found. Ignore it.`);
  }

  console.log(`Label '${label}' is set. Should retry workflow '${workflow_id}'.`);

  // Retrieve latest workflow run and rerun it.
  let response = await github.rest.actions.listWorkflowRuns({
    owner: context.repo.owner,
    repo: context.repo.repo,
    workflow_id: workflow_id,
    branch: context.payload.pull_request.head.ref,
    event: 'push'
  });

  if (!response.data.workflow_runs || response.data.workflow_runs.length === 0) {
    console.log(`ListWorkflowRuns response: ${JSON.stringify(response)}`);
    return core.setFailed(`No runs found for workflow '${workflow_id}'. Just return.`);
  }

  const latestWorkflowRunId = response.data.workflow_runs[0].id;
  console.log(`Last workflow run id: ${latestWorkflowRunId}`);

  try {
    const response = await github.rest.actions.retryWorkflow({
      owner: context.repo.owner,
      repo: context.repo.repo,
      run_id: latestWorkflowRunId
    });

    if (response.status > 200 && response.status < 300) {
      console.log('RetryWorkflow called successfully');
    } else {
      console.log(`Error calling RetryWorkflow. Response: ${JSON.stringify(response)}`);
    }
  } catch (e) {
    console.log(`Ignore error: ${JSON.stringify(e)}`);
  }
};

/**
 * Create new "release" issue when new milestone is created.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<*>}
 */
module.exports.createReleaseIssueForMilestone = async ({ github, context, core }) => {
  const milestone = context.payload.milestone;
  // NOTE: non-breaking space after emoji.
  const issueBody = `:point_right: Use this issue to test milestone [${milestone.title}](${milestone.html_url}) and deploy released tag.
            :point_right: Use 'e2e/run/' labels to run default e2e test.
            :point_right: Use 'e2e/use/' labels to run specific e2e test.
            :point_right: Use 'deploy/' labels to deploy site and documentation.
            :point_right: Use 'deploy/deckhouse/' labels to deploy to channels after creating tag.`;

  const response = await github.rest.issues.create({
    owner: context.repo.owner,
    repo: context.repo.repo,
    title: `Release ${milestone.title}`,
    body: issueBody,
    milestone: milestone.number,
    labels: ['issue/release']
  });

  if (response.status != 201) {
    return core.setFailed(`Create issue failed: ${JSON.stringify(response)}`);
  }
};

/**
 * Find the recent milestone and it's "release" issue. Create new comment and
 * start build-and-test_release workflow with the ID of the created comment.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void>}
 */
module.exports.startBuildAndTestWorkflow = async ({ github, context, core }) => {
  const github_ref = context.ref;

  // Find 10 recently created milestones.
  const query = `
    query($owner:String!, $name:String!) {
      repository(owner:$owner, name:$name){
        milestones(first:10, orderBy:{field:CREATED_AT, direction:DESC}, states:[OPEN]) {
          edges {
            node {
              title
              number
            }
          }
        }
      }
    }`;

  const variables = {
    owner: context.repo.owner,
    name: context.repo.repo
  };

  let result;
  try {
    result = await github.graphql(query, variables);
  } catch (error) {
    if (error.name === 'GraphqlResponseError') {
      console.log('Request:', error.request);
      return core.setFailed(error.message);
    } else {
      // handle non-GraphQL error
      return core.setFailed(`List milestones failed: ${JSON.stringify(error)}`);
    }
  }

  // Find milestone with tag in title.
  const milestones = result.repository.milestones.edges;
  let milestone = null;
  let tagName = '';
  let branchName = '';
  if (context.ref.startsWith('refs/heads/')) {
    branchName = context.ref.replace('refs/heads/', '');
    // Get first milestone with appropriate title.
    for (const m of milestones) {
      if (/^v\d+\.\d+\.\d+/.test(m.node.title)) {
        milestone = m.node;
        break;
      }
    }
  }
  if (context.ref.startsWith('refs/tags/')) {
    // Get milestone with title equal to tag.
    tagName = context.ref.replace('refs/tags/', '');
    for (const m of milestones) {
      if (` ${m.node.title} `.includes(` ${tagName} `)) {
        milestone = m.node;
        break;
      }
    }
  }
  if (!milestone) {
    return core.setFailed(
      'No appropriate milestone found. Create one and push or restart build with label. ${JSON.stringify(result)}'
    );
  }
  console.log(`The milestone is '${milestone.title}' with number ${milestone.number}`);

  // Milestone should has issue to comment. Find it by the specific label.
  let response = await github.rest.issues.listForRepo({
    owner: context.repo.owner,
    repo: context.repo.repo,
    milestone: milestone.number,
    state: 'open',
    labels: [knownLabels['issue-release']]
  });
  if (response.status != 200 || response.data.length < 1) {
    return core.setFailed(`List milestone issues failed: ${JSON.stringify(response)}`);
  }

  const issue = response.data[0];

  // Add issue comment.
  let comment_body = '';
  if (tagName !== '') {
    comment_body = `New tag '${tagName}' is created.`;
  }
  if (branchName !== '') {
    const commitMiniSHA = context.payload.head_commit.id.slice(0, 6);
    const commitUrl = context.payload.head_commit.url;
    const header = `New commit [${commitMiniSHA}](${commitUrl}) in branch '${branchName}':`;
    // Format commit message.
    const mdCodeMarker = '```';
    const commitMsg = `${mdCodeMarker}\n${context.payload.head_commit.message}\n${mdCodeMarker}`;
    comment_body = `${header}\n${commitMsg}\n`;
  }
  console.log('Add issue comment.');
  response = await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
    body: comment_body
  });

  if (response.status != 201) {
    return core.setFailed(`Create issue comment failed: ${JSON.stringify(response)}`);
  }

  // Start 'release-build-and-test' workflow.
  console.log('Start workflow.');
  const issue_id = '' + issue.id;
  const issue_number = '' + issue.number;
  const comment_id = '' + response.data.id;
  response = await github.rest.actions.createWorkflowDispatch({
    owner: context.repo.owner,
    repo: context.repo.repo,
    workflow_id: 'build-and-test_release.yml',
    ref: github_ref,
    inputs: { issue_id, issue_number, comment_id }
  });
  if (response.status < 200 || response.status >= 300) {
    return core.setFailed(`Error calling dispatch. Response: ${JSON.stringify(response)}`);
  }
};
