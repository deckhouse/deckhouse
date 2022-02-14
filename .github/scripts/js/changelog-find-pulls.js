//@ts-check

// Search all merged PRs in passed milestone.
//
// Uses search API to find merged PRs in current milestone and excluding "auto" label.
module.exports = async function ({ github, context }, { milestone }) {
  const repo = `${context.repo.owner}/${context.repo.repo}`;
  const q = `repo:${repo} is:pr is:merged milestone:${milestone} -label:auto`;

  const pulls = await github.paginate(github.rest.search.issuesAndPullRequests, { q });

  // Make JSON compact to pass it further as string
  return pulls.map((p) => ({
    url: p.url,
    number: p.number,
    title: p.title,
    body: p.body,
    state: p.state,
    milestone: {
      number: p.milestone.number,
      title: p.milestone.title,
      state: p.milestone.state
    }
  }));
};
