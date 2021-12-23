Support scripts to use with github-script action in github workflows.

1. Changelog.
2. Run e2e tests.
3. Deploy web and site to stage and test environments.
4. Re-run validation workflows.

Notes:
- Standalone runner can run actions using nodejs v12 and v16.
- github-script uses v12: https://github.com/actions/github-script/blob/v5.0.0/action.yml#L30
- See [compatibility table](https://node.green/) when develop new methods. For example, `replaceAll` is [not available](https://node.green/#ES2021-features--String-prototype-replaceAll) in v12. 
