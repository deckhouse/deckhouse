# helm_lib
ATTENTION! DO NOT CHANGE FILES IN `helm_lib` DIRECTORY DIRECTLY!

Use the [next](https://github.com/deckhouse/lib-helm/blob/main/README.md#working-with-repo) instruction to change `lib-helm`.
To update `lib-helm` in the Deckhouse repo, use the next command (in the repo root):

```bash
make version=DESIRED_VERSION update-lib-helm
```

For example:
```bash
make version=0.0.2 update-lib-helm
```

## Fix lib-helm bugs

ATTENTION!

Do not backport new minor and major releases of `lib-helm` to an existing (already released on the Alpha channel) Deckhouse release. 
These releases may contain breaking changes. Also, you **cannot** use the [backporting mechanism](https://github.com/deckhouse/deckhouse/wiki/Guidelines-for-working-with-PRs#backporting-a-pr)! 
If you need to make a fix, follow the instructions below:
- Release a patch release of `lib-helm` [as follows instruction](https://github.com/deckhouse/lib-helm/blob/main/README.md#backport-fix-in-previous-minor-release-xx).
- Create a new branch from the release branch (release-1.XXX).
- Update `lib-helm` as instructed above.
- Create a PR to the release branch and merge it.
