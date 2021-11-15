//@ts-check

// Gets issue in context, ensures it is PR with milestone, and returns the milestone
module.exports = async ({ github, core, context }) => {
  const { status, data: issue } = await github.rest.issues.get({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
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

  return issue.milestone && issue.milestone.status == "open";
};

function validate(issue) {
  if (!issue.pull_request) {
    return "Not pull request, skip.";
  }

  if (!issue.milestone) {
    return "No milestone, skip.";
  }

  if (issue.milestone.state != "open") {
    return `Milestone ${issue.milestone.title} is not open.`;
  }

  return "";
}
