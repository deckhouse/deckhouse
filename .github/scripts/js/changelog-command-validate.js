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

//@ts-check

// Gets issue in context, ensures it is PR with milestone, and returns the milestone
module.exports = async ({ github, core, context }) => {
  const { status, data: issue } = await github.rest.issues.get({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number
  });

  if (status != 200) {
    core.warning(`GET issue error: status ${status}`);
    return;
  }

  const whatsWrong = validate(issue);
  if (whatsWrong) {
    core.warning(whatsWrong);
    return;
  }

  return issue.milestone;
};

function validate(issue) {
  if (!issue.pull_request) {
    return 'Not pull request, skip.';
  }

  if (!issue.milestone) {
    return 'No milestone, skip.';
  }
  return '';
}
