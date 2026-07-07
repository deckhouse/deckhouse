# Tools Research for GitHub Workflows

Scope: `.github/workflows/*.yml` and `.github/workflows/*.yaml`.

Method:
- `uses:` entries were counted as GitHub Actions and normalized by removing version/tag suffix after `@`.
- `run:` scripts were parsed into command segments (`&&`, `||`, `;`), then the executable token was counted as CLI usage with shell keywords/builtins, variables, flags, and non-tool tokens filtered out.

| Название утилиты | Группа использования | Количество использования |
|---|---|---:|
| `docker/login-action` | GitHub Action | 1415 |
| `actions/github-script` | GitHub Action | 1120 |
| `docker` | CLI | 642 |
| `git` | CLI | 624 |
| `chmod` | CLI | 581 |
| `actions/checkout` | GitHub Action | 515 |
| `chown` | CLI | 504 |
| `bash` | CLI | 477 |
| `hashicorp/vault-action` | GitHub Action | 441 |
| `rm` | CLI | 361 |
| `publish_image` | CLI | 334 |
| `mkdir` | CLI | 297 |
| `ls` | CLI | 210 |
| `actions/upload-artifact` | GitHub Action | 210 |
| `actions-ecosystem/action-add-labels` | GitHub Action | 156 |
| `actions-ecosystem/action-remove-labels` | GitHub Action | 155 |
| `./.github/actions/upload-d8-debug-logs` | GitHub Action | 154 |
| `regctl` | CLI | 91 |
| `pull_push_rmi` | CLI | 90 |
| `version` | CLI | 59 |
| `print` | CLI | 55 |
| `touch` | CLI | 55 |
| `ssh-keygen` | CLI | 54 |
| `curl` | CLI | 37 |
| `werf/actions/converge` | GitHub Action | 36 |
| `cat` | CLI | 31 |
| `werf` | CLI | 31 |
| `crane` | CLI | 30 |
| `push_rmi` | CLI | 30 |
| `mv` | CLI | 28 |
| `webfactory/ssh-agent` | GitHub Action | 27 |
| `make` | CLI | 25 |
| `egrep` | CLI | 24 |
| `promote_with_att` | CLI | 24 |
| `regctl_copy` | CLI | 24 |
| `sign_current_minor` | CLI | 24 |
| `actions/download-artifact` | GitHub Action | 22 |
| `slugify` | CLI | 18 |
| `dawidd6/action-download-artifact` | GitHub Action | 15 |
| `dhctl` | CLI | 14 |
| `python` | CLI | 14 |
| `gh` | CLI | 13 |
| `sleep` | CLI | 13 |
| `rsync` | CLI | 12 |
| `pip` | CLI | 11 |
| `tee` | CLI | 10 |
| `image` | CLI | 6 |
| `actions/setup-python` | GitHub Action | 6 |
| `ln` | CLI | 5 |
| `dorny/paths-filter` | GitHub Action | 5 |
| `werf/actions/build` | GitHub Action | 5 |
| `jq` | CLI | 4 |
| `python3` | CLI | 4 |
| `reproducibility` | CLI | 4 |
| `run` | CLI | 4 |
| `tar` | CLI | 4 |
| `deckhouse/modules-actions/gh` | GitHub Action | 4 |
| `ls-remote` | CLI | 3 |
| `./.github/actions/milestone-changelog` | GitHub Action | 3 |
| `actions/setup-go` | GitHub Action | 3 |
| `peter-evans/create-or-update-comment` | GitHub Action | 3 |
| `Dr.Web` | CLI | 2 |
| `Github` | CLI | 2 |
| `Kaspersky` | CLI | 2 |
| `max_patch` | CLI | 2 |
| `npm` | CLI | 2 |
| `deckhouse/modules-actions/cve_scan` | GitHub Action | 2 |
| `deckhouse/modules-actions/gitleaks` | GitHub Action | 2 |
| `sigstore/cosign-installer` | GitHub Action | 2 |
| `api` | CLI | 1 |
| `iter` | CLI | 1 |
| `iter++` | CLI | 1 |
| `keys` | CLI | 1 |
| `prev_minor` | CLI | 1 |
| `ref_name` | CLI | 1 |
| `which` | CLI | 1 |
| `./.github/workflows/security-scan-images.yml` | GitHub Action | 1 |
| `actions/setup-node` | GitHub Action | 1 |
| `deckhouse/backport-action` | GitHub Action | 1 |
| `deckhouse/changelog-action` | GitHub Action | 1 |
| `github/codeql-action/analyze` | GitHub Action | 1 |
| `github/codeql-action/autobuild` | GitHub Action | 1 |
| `github/codeql-action/init` | GitHub Action | 1 |
| `peter-evans/create-pull-request` | GitHub Action | 1 |
| `peter-evans/slash-command-dispatch` | GitHub Action | 1 |

Уникальных утилит: **85** (GitHub Action: **31**, CLI: **54**).
Проанализировано workflow-файлов: **81**.
