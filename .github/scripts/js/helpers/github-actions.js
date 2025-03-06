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

/**
 * @module GithubActions
 * This module provides utilities for interacting with GitHub Actions workflows
 * @example
 * const githubActions = require('../helpers/github-actions')(github, context);
 */

const WORKFLOW_STATUS_RUNNING = 'in_progress';
const WORKFLOW_STATUS_COMPLETED = 'completed';

const CONCLUSION_STATUS_SUCCESS = 'success';
const CONCLUSION_STATUS_CANCELLED = 'cancelled';
const CONCLUSION_STATUS_FAILURE = 'failure';

const MAX_ITEMS_PER_PAGE = 100;

module.exports = ({ github, context, core }) => {
  /**
   * @param {string} workflowName
   * @param {string} branch
   * @param {string=} status
   * @param {number=} per_page
   *
   * @returns {GithubWorkflow[]}
   */
  async function GetWorkflowsByNameAndStatus(
    workflowName,
    branch,
    status = WORKFLOW_STATUS_RUNNING,
    per_page = MAX_ITEMS_PER_PAGE
  ) {
    const { data } = await github.rest.actions.listWorkflowRunsForRepo({
      owner: context.repo.owner,
      repo: context.repo.repo,
      branch: branch,
      per_page: per_page
    });

    return data.workflow_runs.filter((run) => run.name === workflowName && run.status === status);
  }

  /**
   * @param {string} run_id
   * @param {string} attempt_number
   * @param {string} job_name
   * @param {number=} per_page
   *
   * @returns {GithubWorkflowJob}
   */
  async function GetJobsForWorkflowRunAttempt(run_id, attempt_number, job_name, per_page = MAX_ITEMS_PER_PAGE) {
    const { data } = await github.rest.actions.listJobsForWorkflowRunAttempt({
      owner: context.repo.owner,
      repo: context.repo.repo,
      run_id: run_id,
      attempt_number: attempt_number,
      per_page: per_page
    });

    // search jobs by jobName
    return data.jobs.filter((job) => job.name === job_name);
  }

  /**
   * @param {GithubContextPayload} context
   * @returns {string}
   */
  function GetBranchNameFromContext(context) {
    /** @type {string[]} */
    const splitref = context.payload.ref.split('refs/heads/');
    return splitref[splitref.length - 1];
  }

  /**
   * @param {string} branchName
   * @returns {GithubPullRequest|null}
   */
  async function GetPullRequestByBranchName(branchName) {
    let hasPreviousPage = true;
    let startCursor = null;

    while (hasPreviousPage) {
      const { repository } = await github.graphql(
        `
        query($owner: String!, $repo: String!, $headRefName: String!, $before: String) {
          repository(owner: $owner, name: $repo) {
            pullRequests(headRefName: $headRefName, states: OPEN, last: 5, before: $before) {
              nodes {
                number
                title
                url
                state
              }
              pageInfo {
                hasPreviousPage
                startCursor
              }
            }
          }
        }
      `,
        {
          owner: context.repo.owner,
          repo: context.repo.repo,
          headRefName: branchName,
          before: startCursor
        }
      );

      if (repository.pullRequests.nodes.length === 1) {
        return repository.pullRequests.nodes[0];
      }

      hasPreviousPage = repository.pullRequests.pageInfo.hasPreviousPage;
      startCursor = repository.pullRequests.pageInfo.startCursor;
    }

    return null;
  }

  return {
    WORKFLOW_STATUS_COMPLETED,
    WORKFLOW_STATUS_RUNNING,
    CONCLUSION_STATUS_SUCCESS,
    CONCLUSION_STATUS_CANCELLED,
    CONCLUSION_STATUS_FAILURE,
    GetWorkflowsByNameAndStatus,
    GetJobsForWorkflowRunAttempt,
    GetBranchNameFromContext,
    GetPullRequestByBranchName
  };
};
