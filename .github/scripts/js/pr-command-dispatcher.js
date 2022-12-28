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

const {tryParseAbortE2eCluster} = require("./e2e/slash_workflow_command");
const {commentCommandRecognition} = require("./comments");
const {extractCommandFromComment, reactToComment, startWorkflow} = require("./ci");

/**
 * Detect slash-command in the pull request comment and start
 * another worklflow via workflow_dispatch event.
 *
 * @param {object} inputs
 * @param {object} inputs.github - A pre-authenticated octokit/rest.js client with pagination plugins.
 * @param {object} inputs.context - An object containing the context of the workflow run.
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @returns {Promise<void|*>}
 */
async function runSlashCommandForPullRequest({ github, context, core }) {
  const event = context.payload;
  const comment_id = event.comment.id;
  core.debug(`Event: ${JSON.stringify(event)}`);

  const arg = extractCommandFromComment(event.comment.body)
  if (arg.err) {
    return core.info(`Ignore comment: ${arg.err}.`);
  }

  const commandName = arg.argv[0];
  let slashCommand = dispatchPullRequestCommand({arg, core, context});
  if (!slashCommand) {
    return core.info(`Ignore comment: workflow for command ${commandName} not found.`);
  }

  if (slashCommand.err) {
    return core.setFailed(`Cannot start workflow: ${slashCommand.err}`);
  }

  core.info(`Command detected: ${JSON.stringify(slashCommand)}`);

  const { targetRef, workflow_id } = slashCommand;
  // Git ref is malformed.
  if (!targetRef) {
    core.setFailed('targetRef is missed');
    return await reactToComment({github, context, comment_id, content: 'confused'});
  }

  // Git ref is malformed.
  if (!workflow_id) {
    core.setFailed('workflowID is missed');
    return await reactToComment({github, context, comment_id, content: 'confused'});
  }

  core.info(`Use ref '${targetRef}' for workflow.`);

  // React with rocket emoji!
  await reactToComment({github, context, comment_id, content: 'rocket'});

  // Add new issue comment and start the requested workflow.
  core.info('Add issue comment to report workflow status.');
  let response = await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: event.issue.number,
    body: commentCommandRecognition(event.comment.user.login, commandName)
  });

  if (response.status !== 201) {
    return core.setFailed(`Cannot start workflow: ${JSON.stringify(response)}`);
  }

  return await startWorkflow({github, context, core,
    workflow_id: workflow_id,
    ref: targetRef,
    inputs: {
      comment_id: '' + response.data.id,
      ...slashCommand.inputs
    },
  });
}

/**
 *
 * @param {object} arg - slash command arguments as argv [0] arg is name of command and as lines comment lines
 * @param {object} core - github core object
 * @param {object} context - github core object
 * @return {object}
 */
function dispatchPullRequestCommand({arg, core, context}){
  const { argv, lines } = arg;
  const command = argv[0];
  core.debug(`Command is ${command}`)
  core.debug(`argv is ${JSON.stringify(argv)}`)

  // TODO rewrite to some argv parse library
  const checks = [
    tryParseAbortE2eCluster
  ]

  const prNumber = context.payload.issue.number;
  // Construct head commit ref using pr number.
  const ref = `refs/pull/${ prNumber }/head`;
  core.notice(`Use ref: '${ref}'`)

  for (let i = 0; i < checks.length; i++) {
    const res = checks[i]({argv, lines, core, context, ref})
    if (res !== null) {
      return res;
    }
  }

  return null;
}

module.exports = {
  runSlashCommandForPullRequest,
  dispatchPullRequestCommand
}
