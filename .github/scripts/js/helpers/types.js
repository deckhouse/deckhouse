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
 * @typedef {object} GithubContext
 * @property {object} repo
 * @property {string} repo.owner
 * @property {string} repo.repo
 */

/**
 * @typedef {object} GithubContextPayload
 * @property {object} payload
 * @property {string} payload.eventName
 * @property {string} payload.ref
 */

/**
 * @typedef {object} GithubWorkflowJob
 * @property {string} name
 * @property {number} id
 * @property {number} run_id
 * @property {string} workflow_name
 * @property {string} head_branch
 * @property {string} run_url
 * @property {number} run_attempt
 * @property {string} status
 * @property {string} conclusion
 */

/**
 * @typedef {object} GithubWorkflow
 * @property {number} id
 * @property {string} name
 * @property {string} node_id
 * @property {string} head_branch
 * @property {string} head_sha
 * @property {string} path
 * @property {string} display_title
 * @property {number} run_number
 * @property {string} event
 * @property {string} status
 * @property {string} conclusion
 * @property {number} workflow_id
 * @property {number} check_suite_id
 * @property {string} check_suite_node_id
 * @property {string} url
 * @property {string} html_url
 * @property {Array<object>} pull_requests
 * @property {string} created_at
 * @property {string} updated_at
 * @property {GithubActor} actor
 * @property {number} run_attempt
 * @property {Array<object>} referenced_workflows
 * @property {string} run_started_at
 * @property {GithubActor} triggering_actor
 * @property {string} jobs_url
 * @property {string} logs_url
 * @property {string} check_suite_url
 * @property {string} artifacts_url
 * @property {string} cancel_url
 * @property {string} rerun_url
 * @property {string} previous_attempt_url
 * @property {string} workflow_url
 * @property {GithubCommit} head_commit
 * @property {GithubRepository} repository
 * @property {GithubRepository} head_repository
 */

/**
 * @typedef {object} GithubActor
 * @property {string} login
 * @property {number} id
 * @property {string} node_id
 * @property {string} avatar_url
 * @property {string} gravatar_id
 * @property {string} url
 * @property {string} html_url
 * @property {string} followers_url
 * @property {string} following_url
 * @property {string} gists_url
 * @property {string} starred_url
 * @property {string} subscriptions_url
 * @property {string} organizations_url
 * @property {string} repos_url
 * @property {string} events_url
 * @property {string} received_events_url
 * @property {string} type
 * @property {string} user_view_type
 * @property {boolean} site_admin
 */

/**
 * @typedef {object} GithubCommit
 * @property {string} id
 * @property {string} tree_id
 * @property {string} message
 * @property {string} timestamp
 * @property {object} author
 * @property {object} committer
 */

/**
 * @typedef {object} GithubRepository
 * @property {number} id
 * @property {string} node_id
 * @property {string} name
 * @property {string} full_name
 * @property {boolean} private
 * @property {object} owner
 * @property {string} html_url
 * @property {string} description
 * @property {boolean} fork
 * @property {string} url
 * @property {string} forks_url
 * @property {string} keys_url
 * @property {string} collaborators_url
 * @property {string} teams_url
 * @property {string} hooks_url
 * @property {string} issue_events_url
 * @property {string} events_url
 * @property {string} assignees_url
 * @property {string} branches_url
 * @property {string} tags_url
 * @property {string} blobs_url
 * @property {string} git_tags_url
 * @property {string} git_refs_url
 * @property {string} trees_url
 * @property {string} statuses_url
 * @property {string} languages_url
 * @property {string} stargazers_url
 * @property {string} contributors_url
 * @property {string} subscribers_url
 * @property {string} subscription_url
 * @property {string} commits_url
 * @property {string} git_commits_url
 * @property {string} comments_url
 * @property {string} issue_comment_url
 * @property {string} contents_url
 * @property {string} compare_url
 * @property {string} merges_url
 * @property {string} archive_url
 * @property {string} downloads_url
 * @property {string} issues_url
 * @property {string} pulls_url
 * @property {string} milestones_url
 * @property {string} notifications_url
 * @property {string} labels_url
 * @property {string} releases_url
 * @property {string} deployments_url
 */

/**
 * @typedef {object} GithubPullRequest
 * @property {number} number
 * @property {string} title
 * @property {string} url
 * @property {string} state
 */
