//@ts-check

// Gets issue in context, ensures it is PR with milestone, and returns the milestone.
//
// Using search API to find merged PRs in current milestone and without "auto" label
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
