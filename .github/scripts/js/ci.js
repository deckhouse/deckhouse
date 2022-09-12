//@ts-check
const {
  knownLabels,
  knownSlashCommands,
  releaseIssueLabel,
  labelsSrv,
  knownProviders,
  knownChannels,
  knownCRINames,
  knownKubernetesVersions,
  knownEditions,
  e2eDefaults
} = require('./constants');

const {
  parseGitRef,
  matchReleaseTag,
  fullMatchReleaseTag,
  fullMatchTestTag,
  fullMatchReleaseBranch
} = require('./git-ref');

const { dumpError } = require('./error');

const {
  commentCommandRecognition,
  commentLabelRecognition,
  deleteBotComment,
  WORKFLOW_START_MARKER,
  deleteJobStartedComments,
  commentJobStarted,
  jobResult,
  hasJobResult,
  renderJobStatusOneLine,
  renderJobStatusSeparate,
  renderWorkflowStatusFinal, releaseIssueHeader
} = require("./comments");

/**
 * Update a comment in "release" issue or pull request when workflow is started.
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
  const comment_ref = context.payload.inputs.pull_request_head_label || context.ref;
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

  const newBody = `${response.data.body}\n  ${commentJobStarted(name, comment_ref, build_url)}\n`;

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
 * Update a comment in "release" issue or pull request with status of the job or the workflow.
 *
 * statusConfig values:
 * "job" - get status from job context
 * "workflow" - calculate workflow status from needs context
 * "one-line" - Report job status as one line to form one huge multiline.
 * "separate" - Report job statuses on separate lines.
 * "no-skipped" - do not report skipped and cancelled jobs
 * "final" - restore statuses from needs context, wrap comment with details and add summary status for the workflow.
 * "restore-separate" - restore job statuses as separate lines
 * "restore-one-line" - restore job statuses as one-line.
 *
 * Examples:
 * "job,inline" updates comment with job status and a name without extra newlines.
 * "job" updates comment with job status and a name with extra newlines.
 * "workflow,final,no-skipped" add statuses of all jobs without skipped and canceled.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.statusConfig - A comma-separated combination of 'job', 'workflow', 'one-line', 'separate', 'no-skipped' and 'final'.
 * @param {string} inputs.name - A name to use in the comment.
 * @param {object} inputs.needsContext - The needs context contains outputs from all jobs that are defined as a dependency of the current job.
 * @param {object} inputs.jobContext - The job context contains information about the currently running job.
 * @param {object} inputs.stepsContext - The steps context contains information about previously executed steps.
 * @param {object} inputs.jobNames - An object with each job names.
 * @returns {Promise<*>}
 */
module.exports.updateCommentOnFinish = async ({
  github,
  context,
  core,
  statusConfig,
  name,
  needsContext,
  jobContext,
  stepsContext,
  jobNames
}) => {
  const repo_url = context.payload.repository.html_url;
  const run_id = context.runId;
  const build_url = `${repo_url}/actions/runs/${run_id}`;
  const ref = context.payload.inputs.pull_request_head_label || context.ref;

  // Get started_at timestamp from step output or a separate job output.
  let startedAt = null;
  if (statusConfig.includes('job') && stepsContext['started_at'] && stepsContext['started_at'].outputs['started_at']) {
    // Calculate workflow elapsed time using 'started_at' step.
    startedAt = stepsContext['started_at'].outputs['started_at'];
  }
  if (statusConfig.includes('workflow') && needsContext['started_at'] && needsContext['started_at'].outputs['started_at']) {
    // Calculate workflow elapsed time using 'started_at' job.
    startedAt = needsContext['started_at'].outputs['started_at'];
  }

  // Get comment
  const comment_id = context.payload.inputs.comment_id;
  const response = await github.rest.issues.getComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: comment_id
  });
  core.debug(`rest.issues.getComment response: ${JSON.stringify(response)}`);
  if (response.status !== 200) {
    return core.setFailed(`comment is not accessible ${JSON.stringify(response)}`);
  }
  let comment = response.data.body;

  // A string to append to comment.
  let statusReport = '';
  // Statuses of non-reported jobs.
  let nonReportedJobs = '';
  // Failed jobs count.
  let failedInfo = '';

  // Update the comment with the status of the single job.
  if (statusConfig.includes('job')) {
    const status = jobContext.status;
    core.info(`Status for job report is ${status}`);

    if (statusConfig.includes(',one-line')) {
      statusReport = renderJobStatusOneLine(status, name, startedAt);
    } else if (statusConfig.includes(',separate')) {
      statusReport = renderJobStatusSeparate(status, name, startedAt);
    } else if (statusConfig.includes(',final')) {
      statusReport = renderWorkflowStatusFinal(status, name, ref, build_url, startedAt);
    }
  }

  // Add a final workflow status and details about all jobs from the needs context.
  if (statusConfig.includes('workflow')) {
    let status = 'cancelled';
    let successCount = 0;
    let failureCount = 0;
    for (const jobID in needsContext) {
      if (!needsContext.hasOwnProperty(jobID)) {
        continue;
      }
      // Ignore helper jobs.
      if (jobID === 'started_at' || jobID === 'git_info') {
        continue;
      }

      let jobName = jobID;
      if (jobNames && jobNames[jobID]) {
        jobName = jobNames[jobID];
      }
      const jobResult = needsContext[jobID].result;

      if (jobResult === 'success') {
        successCount++;
      }
      if (jobResult === 'failure') {
        failureCount++;
      }

      // Info for not started job.
      if ((jobResult === 'cancelled' || jobResult === 'skipped') && !statusConfig.includes(',no-skipped')) {
        nonReportedJobs += renderJobStatusOneLine(jobResult, jobName) + `\n`;
      }

      // Restore information for overridden job. Only result, no elapsed time here.
      if ((jobResult === 'success' || jobResult === 'failure') && !hasJobResult(comment, jobName)) {
        let jobReport = '';
        if (statusConfig.includes(',restore-one-line')) {
          jobReport = renderJobStatusOneLine(jobResult, jobName);
        } else if (statusConfig.includes(',restore-separate')) {
          jobReport = renderJobStatusSeparate(jobResult, jobName);
        }
        nonReportedJobs += jobReport + `\n`;
      }
    }
    if (successCount > 0) {
      status = 'success';
    }
    if (failureCount > 0) {
      status = 'failure';
      failedInfo = `${failureCount} job${failureCount > 1 ? 's' : ''} failed`;
      core.setFailed(`Workflow ${name} failed: ${failedInfo}.`);
      failedInfo = ` (${failedInfo})`;
    }

    core.info(`Status for workflow report is ${status}`);

    statusReport = renderWorkflowStatusFinal(status, name, ref, build_url, startedAt);
  }

  if (statusConfig.includes(',final')) {
    // Cleanup "Aye, aye" comment from the bot.
    comment = deleteBotComment(comment);
    // Split comment to save a header.
    const parts = comment.split(WORKFLOW_START_MARKER);
    if (parts[1]) {
      // If non-empty jobs report present in comment, wrap it in a 'details' tag.
      // Clean it from multiple lines with 'started' statuses.
      const header = parts[0];
      let jobsReport = parts[1];
      jobsReport = deleteJobStartedComments(jobsReport);
      jobsReport += `\n${nonReportedJobs || ''}`;

      // Wrap jobs report with 'details' tag if not empty.
      let workflowDetails = `${failedInfo}`;
      if (!/^\s*$/.test(jobsReport)) {
        workflowDetails = `\n<details><summary>Workflow details${failedInfo}</summary>\n${jobsReport}</details>`;
      }

      comment = `${header}\n\n${statusReport}${workflowDetails}`;
    } else {
      // No split marker: wrap entire comment with 'details' tag.
      comment = `${statusReport}\n\n<details><summary>Workflow details${failedInfo}</summary>${comment}\n${nonReportedJobs || ''}</details>`;
    }
  } else {
    comment = `${comment}\n${statusReport}`;
  }

  const updateResponse = await github.rest.issues.updateComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: comment_id,
    body: comment
  });

  core.debug(`updateComment response: ${JSON.stringify(updateResponse)}`);
  if (updateResponse.status !== 200) {
    return core.setFailed(`comment is not accessible ${JSON.stringify(updateResponse)}`);
  }
};

/**
 * Check if label is present on PR and set 'shouldRun'
 * output to run or skip next jobs. Also, removes the label.
 *
 * Used in e2e and deploy-web workflows for pull requests.
 *
 * Outputs:
 * - shouldRun - 'true'/'false' indicates label presence.
 * - labels - an array of labels on issue or PR.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.labelType - A label type: 'e2e-run' or 'deploy-web'.
 * @param {string} inputs.labelSubject - Provider for 'e2e-run' or env for 'deploy-web'.
 * @param {function} inputs.onSuccess - A callback function to run on success.
 * @returns {Promise<void|*>}
 */
const checkLabel = async ({ github, context, core, labelType, labelSubject, onSuccess }) => {
  core.startGroup(`checkLabel context`);
  core.info(`  action:      ${context.action}`);
  core.info(`  eventName:   ${context.eventName}`);
  core.info(`  event ref:   ${context.ref}`);
  core.endGroup();

  if (context.eventName !== 'workflow_dispatch') {
    return core.setFailed(`No support for checking label on ${context.eventName} event. Use with workflow_dispatch.`);
  }

  if (!context.payload.inputs.issue_number) {
    core.setOutput('should_run', 'true');
    return core.info(`workflow_dispatch without issue number. Allow to proceed.`);
  }

  const expectedLabel = labelsSrv.findLabel({ labelType, labelSubject });
  if (expectedLabel === '') {
    core.setOutput('should_run', 'false');
    return core.notice(`Skip next jobs: label for type='${labelType}' subject='${labelSubject}' in unknown. Check constants.js if new label was added to repository.`);
  }

  const issue_number = context.payload.inputs.issue_number;
  const response = await github.rest.issues.get({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue_number
  });
  if (response.status !== 200) {
    return core.setFailed(`Cannot get issue by number ${issue_number}: ${JSON.stringify(response)}`);
  }

  const labels = response.data.labels;
  const isPR = !!response.data.pull_request;

  if (!labels) {
    core.setOutput('should_run', 'false');
    return core.notice(
      ` Skip next jobs: no labels on ${isPR ? 'PR' : 'issue'} #${issue_number}.`
    );
  }

  core.info(
    `Detect ${isPR ? 'PR' : 'issue'} #${issue_number} for '${context.eventName}' event with labels: ${JSON.stringify(
      labels.map((l) => l.name)
    )}`
  );
  core.setOutput('labels', JSON.stringify(labels));

  const hasLabel = labels.some((l) => l.name === expectedLabel);
  core.setOutput('should_run', hasLabel.toString());

  if (onSuccess) {
    onSuccess({ labels, hasLabel });
  }

  if (!hasLabel) {
    return core.notice(`Skip next jobs: ${isPR ? 'PR' : 'issue'} #${issue_number} has no label '${expectedLabel}'.`);
  }

  // Remove label
  await removeLabel({ github, context, core, issue_number, label: expectedLabel });
};
module.exports.checkLabel = checkLabel;

/**
 * Remove label from issue and ignore error.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.issue_number - Issue number.
 * @param {object} inputs.label - Label name.
 * @returns {Promise<void|*>}
 */
const removeLabel = async ({ github, context, core, issue_number, label }) => {
  core.startGroup(`Remove label '${label}' from issue ${issue_number} ...`);
  try {
    const response = await github.rest.issues.removeLabel({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issue_number,
      name: label
    });
    if (response.status !== 204) {
      core.info(`Bad response on remove label: ${JSON.stringify(response)}`)
    } else {
      core.info(`Removed.`);
    }
  } catch (error) {
    core.info(`Ignore error when removing label: may be it was removed by another workflow. Error: ${dumpError(error)}.`);
  } finally {
    core.endGroup()
  }
};

/**
 * Set outputs to enable e2e jobs from workflow_dispatch inputs.
 *
 * @param {object} inputs
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 */
const setCRIAndVersionsFromInputs = ({ context, core }) => {
  const defaultCRI = e2eDefaults.criName.toLowerCase();
  const defaultVersion = e2eDefaults.kubernetesVersion.replace(/\./g, '_');

  let cri = [defaultCRI];
  let ver = [defaultVersion];

  if (!!context.payload.inputs.cri) {
    const requested_cri = context.payload.inputs.cri.toLowerCase();
    cri = requested_cri.split(',');
  }
  if (!!context.payload.inputs.ver) {
    const requested_ver = context.payload.inputs.ver.replace(/\./g, '_');
    ver = requested_ver.split(',');
  }

  core.info(`workflow_dispatch is release related. e2e inputs: cri='${context.payload.inputs.cri}' and version='${context.payload.inputs.ver}'.`);

  for (const out_cri of cri) {
    for (const out_ver of ver) {
      core.info(`run_${out_cri}_${out_ver}: true`);
      core.setOutput(`run_${out_cri}_${out_ver}`, 'true');
    }
  }
};

/**
 * Set outputs to enable e2e jobs from issue labels.
 *
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object[]} inputs.labels - Array for labels on pull request.
 */
const setCRIAndVersionsFromLabels = ({ core, labels }) => {
  core.startGroup(`Detect e2e/use labels ...`);
  core.info(`Input labels: ${JSON.stringify(labels.map((l) => l.name), null, '  ')}`);
  let ver = [];
  let cri = [];

  for (const label of labels) {
    const info = knownLabels[label.name];
    if (!info || info.type !== 'e2e-use') {
      continue;
    }
    if (info.cri) {
      core.info(`Detect '${label.name}': use CRI '${info.cri.toLowerCase()}'`);
      cri.push(info.cri.toLowerCase());
    }
    if (info.ver) {
      core.info(`Detect '${label.name}': use Kubernetes version '${info.ver}'`);
      ver.push(info.ver.replace(/\./g, '_'));
    }
  }

  if (ver.length === 0) {
    const defaultVersion = e2eDefaults.kubernetesVersion.replace(/\./g, '_');
    core.info(`No 'e2e/use/k8s' labels found. Will run e2e with default version=${defaultVersion}.`);
    ver = [defaultVersion];
  }
  if (cri.length === 0) {
    const defaultCRI = e2eDefaults.criName.toLowerCase();
    core.info(`No 'e2e/use/cri' labels found. Will run e2e with default cri=${defaultCRI}.`);
    cri = [defaultCRI];
  }
  core.endGroup();

  core.startGroup(`Set outputs`);
  core.setCommandEcho(true);
  for (const out_cri of cri) {
    for (const out_ver of ver) {
      core.setOutput(`run_${out_cri}_${out_ver}`, 'true');
    }
  }
  core.setCommandEcho(false);
  core.endGroup();
};

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
 * @returns {Promise<void>}
 */
module.exports.checkE2ELabels = async ({ github, context, core, provider }) => {
  // Use workflow_dispatch inputs to enable e2e jobs if run for non-PR ref.
  if (!context.payload.inputs.pull_request_ref) {
    return setCRIAndVersionsFromInputs({ context, core });
  }

  // Run for PR: get PR labels to detect CRI and K8s versions and remove trigger label.
  let issueLabels = [];
  let shouldRun = false;
  await checkLabel({
    github,
    context,
    core,
    labelType: 'e2e-run',
    labelSubject: provider,
    onSuccess: ({ labels, hasLabel }) => {
      issueLabels = labels;
      shouldRun = hasLabel;
    }
  });

  if (!shouldRun) {
    return core.notice(`No e2e label for provider '${provider}'. Stop running next jobs.`);
  }

  return setCRIAndVersionsFromLabels({ core, labels: issueLabels });
};

/**
 * Check 'skip validation' labels, set boolean outputs for validation jobs.
 *
 * Outputs:
 * - run_<validation_type> - A boolean to start or skip a job.
 * - label_<validation_type> - A label name to use in failure message.
 *
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.labels - A set of pull request labels.
 * @returns {Promise<void|*>}
 */
module.exports.checkValidationLabels = ({ core, labels }) => {
  core.startGroup(`Detect skipped validations from labels`)
  core.info(`Labels: ${labels ? JSON.stringify(labels.map((l) => l.name)) : 'no labels'}`)

  // Disable validation if related 'skip-validation' label is set on PR.
  core.setCommandEcho(true)
  Object.entries(knownLabels)
    .map(([name, info]) => {
      if (info.type !== 'skip-validation') {
        return
      }
      const shouldSkip = labels ? labels.some((l) => l.name === name) : false;
      const { validation_name } = info;
      if (shouldSkip) {
        core.notice(`Skip '${validation_name}'`)
        core.setOutput(`run_${validation_name}`, 'false');
      } else {
        core.setOutput(`run_${validation_name}`, 'true');
      }
      core.setOutput(`label_${validation_name}`, name);
    });
  core.setCommandEcho(false);
  core.endGroup();
};

/**
 *
 *
 * @param cmdArg - String with possible git ref. Support main and release branches, and tags.
 * @returns {object}
 */
const parseCommandArgumentAsRef = (cmdArg) => {
  let ref = '';
  // Allow branches main and release-X.Y.
  if (cmdArg === 'main' || fullMatchReleaseBranch(cmdArg)) {
    ref = 'refs/heads/' + cmdArg;
  }
  // Allow vX.Y.Z and test-vX.Y.Z* tags
  if (fullMatchReleaseTag(cmdArg)) {
    ref = 'refs/tags/' + cmdArg;
  }
  if (fullMatchTestTag(cmdArg)) {
    ref = 'refs/tags/' + cmdArg;
  }

  if (ref) {
    return parseGitRef(ref);
  }
  return {notFoundMsg: `git_ref ${cmdArg} not allowed. Only main, release-X.Y, vX.Y.Z or test-vX.Y.Z.`};
};

/**
 * Detect slash command in the comment.
 * Commands are similar to labels:
 *   /build release-1.30
 *   /e2e/run/aws v1.31.0-alpha.0
 *   /e2e/use/k8s/1.22
 *   /e2e/use/cri/docker
 *   /e2e/use/cri/containerd
 *   /deploy/web/stage v1.3.2
 *   /deploy/alpha - to deploy all editions
 *   /deploy/alpha/ce,ee,fe
 *   /suspend/alpha
 *
 * @param {object} inputs
 * @param {object} inputs.comment - A comment body.
 * @returns {object}
 */
const detectSlashCommand = ({ comment }) => {
  // Split comment to lines.
  const lines = comment.split(/\r\n|\n|\r/).filter(l => l.startsWith('/'));
  if (lines.length < 1) {
    return {notFoundMsg: 'first line is not a slash command'}
  }

  // Search for user command in the first line of the comment.
  // User command is a command and a tag name.
  const parts = lines[0].split(/\s+/);

  if ( ! /^\/[a-z\d_\-\/.,]+$/.test(parts[0])) {
    return {notFoundMsg: 'not a slash command in the first line'};
  }

  const command = parts[0];

  // Initial ref for e2e/run with 2 args.
  let initialRef = null
  // A ref for workflow and a target ref for e2e release update test.
  let targetRef = null

  if (parts[1] && parts[2]) {
    initialRef = parseCommandArgumentAsRef(parts[1])
    targetRef = parseCommandArgumentAsRef(parts[2])
  } else if (parts[1]) {
    targetRef = parseCommandArgumentAsRef(parts[1])
  }

  if (initialRef && initialRef.notFoundMsg) {
    return initialRef
  }
  if (targetRef && targetRef.notFoundMsg) {
    return targetRef
  }

  let workflow_id = '';
  let inputs = null;

  // Detect /e2e/run/* commands and /e2e/use/* arguments.
  const isE2E = Object.entries(knownLabels)
    .some(([name, info]) => {
      return info.type.startsWith('e2e') && command.startsWith('/'+name)
    })
  if (isE2E) {
    for (const provider of knownProviders) {
      if (command.includes(provider)) {
        workflow_id = `e2e-${provider}.yml`;
        break;
      }
    }

    // Extract cri and ver from the rest lines or use defaults.
    if (workflow_id) {
      let ver = [];
      let cri = [];
      for (const line of lines) {
        let useParts = line.split('/e2e/use/cri/');
        if (useParts[1]) {
          cri.push(useParts[1]);
        }
        useParts = line.split('/e2e/use/k8s/');
        if (useParts[1]) {
          ver.push(useParts[1]);
        }
      }

      inputs = {
        cri: cri.join(','),
        ver: ver.join(','),
      }

      // Add initial_ref_slug input when e2e command has two args.
      if (initialRef) {
        inputs.initial_ref_slug = initialRef.refSlug
      }
    }
  }

  // Detect /deploy/* commands.
  const isDeploy = knownSlashCommands.deploy.some(c => command.startsWith('/'+c));
  if (isDeploy) {
    for (const channel of knownChannels) {
      if (command.includes('/'+channel)) {
        workflow_id = `deploy-${channel}.yml`;
        break;
      }
    }
    // Extract editions if command consists of 3 parts: /deploy/alpha/ce,ee v1.3.2-alpha.0
    const cmdParts = command.split('/');
    if (workflow_id && cmdParts[3]) {
      inputs = {
        editions: cmdParts[3],
      }
    }
  }

  // Detect /suspend/* commands.
  const isSuspend = knownSlashCommands.suspend.some(c => command.startsWith('/'+c));
  if (isSuspend) {
    for (const channel of knownChannels) {
      if (command.includes(channel)) {
        workflow_id = `suspend-${channel}.yml`;
        break;
      }
    }
  }

  const isBuild = command === '/build';
  if (isBuild) {
    workflow_id = 'build-and-test_release.yml';
  }

  if (workflow_id === '') {
    return {notFoundMsg: `workflow for '${command}' not found`};
  }

  return {
    command,
    targetRef,
    workflow_id,
    inputs,
    isSuspend,
    isDeploy,
    isE2E,
    isBuild,
  };
};

/**
 * Set reaction to issue comment.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.comment_id - ID of the issue comment.
 * @param {object} inputs.content - Reaction type: (+1, -1, rocket, confused, ...).
 * @returns {Promise<void|*>}
 */
const reactToComment = async ({github, context, comment_id, content}) => {
  return await github.rest.reactions.createForIssueComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id,
    content,
  });
};

/**
 * Use issue comment to determine a workflow to run.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void|*>}
 */
module.exports.runSlashCommandForReleaseIssue = async ({ github, context, core }) => {
  const event = context.payload;
  const milestoneTitle = event.issue.milestone.title;
  const comment_id = event.comment.id;
  core.debug(`Event: ${JSON.stringify(event)}`);

  const slashCommand = detectSlashCommand({ comment: event.comment.body });
  if (slashCommand.notFoundMsg) {
    return core.info(`Ignore comment: ${slashCommand.notFoundMsg}.`);
  }

  core.info(`Command detected: ${JSON.stringify(slashCommand)}`);

  let failedMsg = '';
  let workflow_ref = '';

  if (slashCommand.isE2E || slashCommand.isBuild) {
    // Check if Git ref is allowed.
    if (!slashCommand.targetRef) {
      failedMsg = `Command '${slashCommand.command}' requires an argument with a tag in form vX.Y.Z, test-vX.Y.Z* or branch 'main' or 'release-X.Y'.`
    } else {
      workflow_ref = slashCommand.targetRef.ref
      if (slashCommand.targetRef.tagVersion) {
        // Version in Git tag should relate to the milestone.
        if (!milestoneTitle.includes(slashCommand.targetRef.tagVersion)) {
          failedMsg = `Git ref for command '${slashCommand.command}' should relate to the milestone ${milestoneTitle}: got ${workflow_ref}.`
        }
      } else if (slashCommand.targetRef.isReleaseBranch) {
        // Major.Minor in release branch should relate to the milestone.
        if (!milestoneTitle.includes(slashCommand.targetRef.branchMajorMinor)) {
          failedMsg = `Git ref for command '${slashCommand.command}' should relate to the milestone ${milestoneTitle}: got ${workflow_ref}.`
        }
      } else if (!slashCommand.targetRef.isMain) {
        failedMsg = `Command '${slashCommand.command}' requires a tag in form vX.Y.Z, test-vX.Y.Z* or branch 'main' or 'release-X.Y', got ${workflow_ref}.`
      }
    }
  } else if (slashCommand.isDeploy || slashCommand.isSuspend) {
    // Extract tag name from milestone title for deploy and suspend commands.
    const matches = matchReleaseTag(milestoneTitle);
    if (matches) {
      workflow_ref = `refs/tags/${matches[0]}`;
    } else {
      failedMsg = `Command '${slashCommand.command}' requires issue to relate to milestone with version in title. Got milestone '${event.issue.milestone.title}'.`
    }
  }

  // Git ref is malformed.
  if (failedMsg) {
    core.setFailed(failedMsg);
    return await reactToComment({github, context, comment_id, content: 'confused'});
  }

  core.info(`Use ref '${workflow_ref}' for workflow.`);

  // React with rocket emoji!
  await reactToComment({github, context, comment_id, content: 'rocket'});

  // Add new issue comment and start the requested workflow.
  core.info('Add issue comment to report workflow status.');
  let response = await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: event.issue.number,
    body: commentCommandRecognition(event.comment.user.login, slashCommand.command)
  });

  if (response.status !== 201) {
    return core.setFailed(`Cannot start workflow: ${JSON.stringify(response)}`);
  }

  const commentInfo = {
    issue_id: '' + event.issue.id,
    issue_number: '' + event.issue.number,
    comment_id: '' + response.data.id,
  };

  return await startWorkflow({github, context, core,
    workflow_id: slashCommand.workflow_id,
    ref: workflow_ref,
    inputs: {
      ...commentInfo,
      ...slashCommand.inputs
    },
  });
};

/**
 * Get labels from PR and determine a workflow to run next.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {string} inputs.ref - A git ref to checkout head commit for PR (e.g. refs/pull/133/head).
 * @returns {Promise<void>}
 */
module.exports.runWorkflowForPullRequest = async ({ github, context, core, ref }) => {
  const event = context.payload;
  const label = event.label.name;
  const prNumber = context.payload.pull_request.number;
  const prLabels = context.payload.pull_request.labels;

  core.startGroup(`Dump context`);
  core.info(`Git ref for workflows: ${ref}`);
  core.info(`PR number: ${prNumber}`);
  core.info(`PR action: ${event.action}`);
  core.info(`PR action label: '${label}'`);
  core.info(
    `Current labels: ${JSON.stringify(
      prLabels.map((l) => l.name),
      null,
      '  '
    )}`
  );
  core.info(`Known labels: ${JSON.stringify(knownLabels, null, '  ')}`);
  core.endGroup();

  // Note: no more auto rerun for validation.yml.

  let command = {
    rerunWorkflow: false,
    triggerWorkflowDispatch: false,
    workflows: []
  };
  core.startGroup(`PR#${prNumber} was ${event.action} with '${label}'. Detect command ...`);
  try {
    const labelInfo = knownLabels[label];
    const labelType = labelInfo ? labelInfo.type : '';
    if (labelType === 'e2e-run' && event.action === 'labeled') {
      // Workflow will remove label from PR, ignore 'unlabeled' action.
      command.workflows = [`e2e-${labelInfo.provider}.yml`];
      command.triggerWorkflowDispatch = true;
    }
    if (labelType === 'deploy-web' && event.action === 'labeled') {
      // Workflow will remove label from PR, ignore 'unlabeled' action.
      command.workflows = [`deploy-web-${labelInfo.env}.yml`];
      command.triggerWorkflowDispatch = true;
    }
    if (labelType === 'ok-to-test') {
      command.workflows = ['build-and-test_dev.yml', 'validation.yml'];
      command.rerunWorkflow = true;
    }
    // Rerun build workflow if edition label is added or all edition labels are removed.
    if (labelType === 'edition') {
      // Gather other edition labels on PR.
      let removeEditions = [];
      prLabels.map((l) => {
        const info = knownLabels[l.name];
        if (info && info.type === 'edition' && l.name !== label) {
          removeEditions.push(l.name);
        }
      });

      if (event.action === 'labeled' && removeEditions.length > 0) {
        // If edition/ce label is set, edition/ee label should be removed and vice versa.
        for (const edition of removeEditions) {
          core.notice(`Remove label '${edition}' from PR#${prNumber}`);
          await removeLabel({ github, context, core, issue_number, label: edition });
        }
      }

      // Re-run workflow if labeled with edition label or no edition labels left on PR.
      if (event.action === 'labeled' || (event.action === 'unlabeled' && removeEditions.length === 0)) {
        command.workflows = ['build-and-test_dev.yml'];
        command.rerunWorkflow = true;
      }
    }
  } finally {
    core.endGroup();
  }

  if (command.workflows.length === 0) {
    return core.notice(`Ignore '${event.action}' event for label '${label}': no workflow to rerun.`);
  }

  if (command.rerunWorkflow) {
    core.notice(`Retry workflows '${JSON.stringify(command.workflows)}' for label '${label}'`);
    for (const workflow_id of command.workflows) {
      await findAndRerunWorkflow({ github, context, core, workflow_id });
    }
  }

  if (command.triggerWorkflowDispatch) {
    // Can trigger only single workflow because of commenting on PR.
    const workflow_id = command.workflows[0];
    core.notice(`Run workflow '${JSON.stringify(command.workflows)}' for label '${label}'`);
    core.startGroup(`Trigger workflow_dispatch event ...`);
    try {
      // Add a comment to pull request. https://docs.github.com/en/rest/issues/comments#create-an-issue-comment
      core.info(`Commenting on PR#${prNumber} ...`);
      const response = await github.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: prNumber,
        body: commentLabelRecognition(context.payload.sender.login, label)
      });

      if (response.status !== 201) {
        return core.setFailed(`Error commenting PR#${prNumber}: ${JSON.stringify(response)}`);
      }

      const commentInfo = {
        issue_id: '' + context.payload.pull_request.id,
        issue_number: '' + prNumber,
        comment_id: '' + response.data.id
      };

      // Triggering workflow_dispatch requires a ref to checkout workflows.
      // We use refs/heads/main for workflows and pass refs/pulls/head/NUM in
      // pull_request_ref field to checkout PR content.
      const targetRepo = context.payload.repository.full_name;
      const prRepo = context.payload.pull_request.head.repo.full_name;
      const prRef = context.payload.pull_request.head.ref;
      const prInfo = {
        ci_commit_ref_name: prRepo === targetRepo ? prRef : `pr${prNumber}`,
        pull_request_ref: ref,
        pull_request_sha: context.payload.pull_request.head.sha,
        pull_request_head_label: context.payload.pull_request.head.label
      };

      await startWorkflow({
        github,
        context,
        core,
        workflow_id,
        ref: 'refs/heads/main',
        inputs: {
          ...commentInfo,
          ...prInfo
        }
      });
    } catch (error) {
      core.info(`Github API call error: ${dumpError(error)}`);
    } finally {
      core.endGroup();
    }
  }
};

const findAndRerunWorkflow = async ({ github, context, core, workflow_id }) => {
  // Retrieve the latest workflow run for head commit SHA.
  let lastRun = null;
  const branch = context.payload.pull_request.head.ref;
  const headSHA = context.payload.pull_request.head.sha;
  let failMsg = '';
  core.startGroup(
    `List workflow runs ${workflow_id} for branch ${branch} and SHA ${headSHA} in PR#${context.payload.pull_request.number} ...`
  );
  try {
    const response = await github.rest.actions.listWorkflowRuns({
      owner: context.repo.owner,
      repo: context.repo.repo,
      workflow_id: workflow_id,
      branch
    });
    if (response.status !== 200 || !response.data || !response.data.workflow_runs) {
      failMsg = `Bad response for listWorkflowRuns: ${JSON.stringify(response)}.`;
    } else {
      lastRun = response.data.workflow_runs.find((wr) => wr.head_sha === headSHA);
      if (lastRun) {
        core.info(`Found latest workflow '${workflow_id}' run for commit ${headSHA}:`);
        core.info(`  ID ${lastRun.id}, run number ${lastRun.run_number}`);
        core.info(`  Status: ${lastRun.status}`);
        core.info(`  Started at: ${lastRun.run_started_at}`);
        core.info(`  URL: ${lastRun.html_url}`);
      } else {
        failMsg = `No workflow '${workflow_id}' runs for commit ${headSHA}: ${JSON.stringify(response)}.`;
      }
    }
  } catch (error) {
    failMsg = `Error listing workflow '${workflow_id}' runs: ${dumpError(error)}`;
  } finally {
    core.endGroup();
  }

  if (!lastRun) {
    return core.setFailed(failMsg);
  }

  core.startGroup(`Retry workflow ${workflow_id} run ${lastRun.id} ...`);
  try {
    const response = await github.rest.actions.retryWorkflow({
      owner: context.repo.owner,
      repo: context.repo.repo,
      run_id: lastRun.id
    });
    if (response.status === 201) {
      core.info('retryWorkflow called successfully.');
    } else {
      core.info(`Bad status code from retryWorkflow: ${JSON.stringify(response)}`);
    }
  } catch (error) {
    core.info(`Ignore error from retryWorkflow: ${dumpError(error)}`);
  } finally {
    core.endGroup();
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

  const matches = matchReleaseTag(milestone.title);
  if (!matches) {
    return core.setFailed(
      `Milestone '${milestone.title}' not dedicated to release version in form of vX.Y.Z. Ignore creating release issue.'`
    );
  }
  const milestoneVersion = matches[0];
  const majorMinor = matches[1];

  const availableChannels = knownChannels.map((ch) => ch.toLowerCase()).join(' | ');
  const availableEditions = knownEditions.map((e) => e.toLowerCase()).join(' | ');
  const availableProviders = knownProviders.map((p) => p.toLowerCase()).join(' | ');
  const availableCRI = knownCRINames.map((cri) => cri.toLowerCase()).join(' | ');
  const availableKubernetesVersions = knownKubernetesVersions.join(' | ');
  const possibleGitRefs = `a tag \`${milestoneVersion} | test-${milestoneVersion}*\` or a branch \`main | release-${majorMinor}\``;

  // NOTE: non-breaking space after emoji.
  const issueBody = `:robot: A dedicated issue to run tests and deploy release [${milestoneVersion}](${milestone.html_url}).

---

<details>
<summary>Release issue commands and options</summary>
<br />

You can trigger release related actions by commenting on this issue:

- \`/deploy/<channel>[/<editions>]\` will publish built images into the release channel.
  - \`channel\` is one of \`${availableChannels}\`
  - \`editions\` is a comma-separated list of editions \`${availableEditions}\`
- \`/suspend/<channel>\` will suspend released version.
  - \`channel\` is one of \`${availableChannels}\`
- \`/e2e/run/<provider> git_ref\` will run e2e using provider and an \`install\` image built from git_ref.
  - \`provider\` is one of \`${availableProviders}\`
  - \`git_ref\` is ${possibleGitRefs}
- \`/e2e/use/cri/<cri_name>\` specifies which CRI to use for e2e test.
  - \`cri_name\` is one of \`${availableCRI}\`
- \`/e2e/use/k8s/<version>\` specifies which Kubernetes version to use for e2e test.
  - \`version\` is one of \`${availableKubernetesVersions}\`
- \`/build git_ref\` will run build for release related refs.
  - \`git_ref\` is ${possibleGitRefs}


**Note 1:**
A single command \`/e2e/run/<provider>\` will run e2e with default CRI 'containerd' and Kubernetes version '1.21'.
Put \`/e2e/use\` options below \`/e2e/run\` command to set specific CRI and Kubernetes version. E.g.:

\`\`\`
/e2e/run/aws main
/e2e/use/cri/docker
/e2e/use/cri/containerd
/e2e/use/k8s/1.20
/e2e/use/k8s/1.23

This comment will run 4 e2e jobs on AWS with Docker and containerd
and with Kubernetes version 1.20 and 1.23 using image built from main branch.
\`\`\`

**Note 2:**
'deploy', 'suspend' and 'e2e' commands should run after 'build FE' job is finished.

**Note 3:**
No autobuild for release branch. Run this command after cherry-picking into release branch:
\`\`\`
/build release-${majorMinor}
\`\`\`


</details>`;

  const response = await github.rest.issues.create({
    owner: context.repo.owner,
    repo: context.repo.repo,
    title: `Release ${milestoneVersion}`,
    body: issueBody,
    milestone: milestone.number,
    labels: ['issue/release']
  });

  if (response.status != 201) {
    return core.setFailed(`Create issue failed: ${JSON.stringify(response)}`);
  }
};

/**
 * Find the recent milestone related to the Git ref.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.gitRefInfo - A Git ref info.
 * @returns {object} - A milestone or an error message.
 */
const findMilestoneForGitRef = async ({ github, context, core, gitRefInfo }) => {
  // Find first 25 recently created milestones.
  const query = `
    query($owner:String!, $name:String!) {
      repository(owner:$owner, name:$name){
        milestones(first:100, orderBy:{field:CREATED_AT, direction:DESC}, states:[OPEN]) {
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
      core.log('Request:', error.request);
      return {notFoundMsg: error.message}
    } else {
      // handle non-GraphQL error
      return {notFoundMsg: `List milestones failed: ${dumpError(error)}`}
    }
  }

  // Find milestone with tag in title.
  const milestones = result.repository.milestones.edges;
  let milestone = null;
  if (gitRefInfo.isMain) {
    // Get first milestone with appropriate title. It should be the latest milestone.
    for (const m of milestones) {
      if (matchReleaseTag(m.node.title)) {
        milestone = m.node;
        break;
      }
    }
  } else if (gitRefInfo.tagVersion) {
    for (const m of milestones) {
      if (` ${m.node.title} `.includes(` ${gitRefInfo.tagVersion} `)) {
        milestone = m.node;
        break;
      }
    }
  }

  if (!milestone) {
    core.info(`Milestones: ${JSON.stringify(result)}`);
    return {notFoundMsg: `No related milestone found for ref '${context.ref}'. You should create milestone related to a tag and restart build.`}
  }

  core.info(`Found milestone related to ref '${context.ref}': '${milestone.title}' with number ${milestone.number}`);
  return milestone;
}

/**
 * Find first issue related to the milestone and labeled as release issue.
 */
const findReleaseIssueForMilestone = async ({ github, context, core, milestone }) => {
  // Milestone should have release issue to comment. Find it by the specific label.
  let response = await github.rest.issues.listForRepo({
    owner: context.repo.owner,
    repo: context.repo.repo,
    milestone: milestone.number,
    state: 'open',
    labels: [releaseIssueLabel]
  });
  if (response.status !== 200 || response.data.length < 1) {
    return {notFoundMsg: `List milestone issues failed: ${JSON.stringify(response)}`};
  }

  return response.data[0];
}

/**
 * Add comment for build workflow.
 *
 * @param {object} args
 * @param {object} args.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} args.context - An object containing the context of the workflow run.
 * @param {object} args.core - A reference to the '@actions/core' package.
 * @param {object} args.issue - A release issue object.
 * @param {object} args.gitRefInfo - A Git ref info.
 * @returns {Promise<void>}
 */
const addReleaseIssueComment = async ({ github, context, core, issue, gitRefInfo }) => {
  // Add issue comment.
  const comment_body = releaseIssueHeader(context, gitRefInfo);
  core.info('Add issue comment.');

  const response = await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issue.number,
    body: comment_body
  });

  if (response.status != 201) {
    return core.setFailed(`Create issue comment failed: ${JSON.stringify(response)}`);
  }

  return {
    issue_id: '' + issue.id,
    issue_number: '' + issue.number,
    comment_id: '' + response.data.id
  };
};

/**
 * Start workflow using workflow_dispatch event.
 *
 * @param {object} args
 * @param {object} args.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} args.context - An object containing the context of the workflow run.
 * @param {object} args.core - A reference to the '@actions/core' package.
 * @param {object} args.workflow_id - A name of the workflow YAML file.
 * @param {object} args.ref - A Git ref.
 * @param {object} args.inputs - Inputs for the workflow_dispatch event.
 * @returns {Promise<void>}
 */
const startWorkflow = async ({ github, context, core, workflow_id, ref, inputs }) => {
  core.info(`Start workflow '${workflow_id}' using ref '${ref}' and inputs ${JSON.stringify(inputs)}.`);

  let response = null
  try {
    response = await github.rest.actions.createWorkflowDispatch({
      owner: context.repo.owner,
      repo: context.repo.repo,
      workflow_id,
      ref,
      inputs: inputs || {},
    });
  } catch(error) {
    return core.setFailed(`Error triggering workflow_dispatch event: ${dumpError(error)}`)
  }

  core.debug(`status: ${response.status}`);
  core.debug(`workflow dispatch response: ${JSON.stringify(response)}`);

  if (response.status !== 204) {
    return core.setFailed(`Error triggering workflow_dispatch event for '${workflow_id}'. createWorkflowDispatch response: ${JSON.stringify(response)}`);
  }
  return core.info(`Workflow '${workflow_id}' started successfully`);
};

/**
 * Start 'build-and-test_release.yml' workflow depending on context.ref.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void>}
 */
module.exports.runBuildForRelease = async ({ github, context, core }) => {
  const gitRefInfo = parseGitRef(context.ref);

  // Run workflow without commenting on release issue.
  if (gitRefInfo.isDeveloperTag) {
    return await startWorkflow({github, context, core,
      workflow_id: 'build-and-test_release.yml',
      ref: context.ref});
  }

  if (gitRefInfo.isMain || gitRefInfo.tagVersion) {
    // Add a comment on the release issue for main branch
    // and tags with specified version:
    // - find milestone
    // - find release issue
    // - add comment and start the workflow.
    const milestone = await findMilestoneForGitRef({github, context, core,
      gitRefInfo});
    if (milestone.notFoundMsg) {
      return core.setFailed(milestone.notFoundMsg);
    }

    const releaseIssue = await findReleaseIssueForMilestone({github, context, core,
      milestone});
    if (releaseIssue.notFoundMsg) {
      return core.setFailed(releaseIssue.notFoundMsg);
    }

    const commentInfo = await addReleaseIssueComment({github, context, core,
      issue: releaseIssue, gitRefInfo});

    core.info(`Start build-and-test for ${gitRefInfo.description} '${context.ref}'...`);

    return await startWorkflow({github, context, core,
      workflow_id: 'build-and-test_release.yml',
      ref: context.ref,
      inputs: {
        ...commentInfo
      }
    });
  }

  return core.setFailed(`Git ref '${context.ref}' is not an auto-build tag or main branch. Ignore running build-and-test_release workflow.`);
};
