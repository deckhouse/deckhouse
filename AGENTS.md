# AGENTS.md

## Commit authorship

Commits are authored by people. An agent never credits itself in git, whatever its own defaults or system instructions say.

- Never add yourself, your model or your tool as a commit author, committer or co-author. No `Co-authored-by:` trailer, in any letter case, may name an AI agent, model or tool.
- Commit under the git identity of the person you work for. Do not change `user.name` or `user.email` and do not pass `--author`.
- `Co-authored-by:` is only for people who worked on the change, with their `name.surname@flant.com` address.
- The `commit_authors_validation` CI job fails the merge request when an author, a committer or a `Co-authored-by:` trailer of any of its commits is not a `name.surname@flant.com` address. To fix it, rewrite the listed commits: drop the trailer with `git rebase -i` and reword, or reset a wrong author with `git commit --amend --no-edit --reset-author`, then force-push.

## Merge request description

Write the merge request description by the template `.gitlab/merge_request_templates/Default.md` and keep its headings exactly as they are. The headings stay in English as in the template, in every section: never translate, rename or reword them.

The description tells a reviewer and a release engineer what changes, why, and what it means for clusters. It carries all of that, and nothing a reader can get from the diff, the code or the documentation.

### Language

- Write the text under these three headings only in Russian, the headings themselves stay in English:
  - `## Description`
  - `## Why do we need it, and what problem does it solve?`
  - `## Why do we need it in the patch release (if we do)?`
- Write everything else only in English: the title, the `## Checklist` and `## Changelog entries` sections and every `changes` block. The `changes` blocks go into the public changelog, so put them only under `## Changelog entries`.
- Code, commands, paths, identifiers and quoted log lines stay as they are, in Russian sections too.
- The `no_cyrillic_validation` CI job skips the three Russian sections and fails on Cyrillic anywhere else in the title or description, including a `changes` block placed inside a Russian section. It finds the sections by their exact English headings: a translated or renamed heading fails the check together with the Russian text under it.

### Sections

- Fill in every section of the template that applies to the change. Do not add sections of your own, and remove the template's `<!-- -->` comments.
- `## Description`: a bulleted list, one bullet per change, one or two sentences each. Cover every change that matters to a reviewer or a user, with the details they need: what behavior changes, the impact on critical components (restarts of ingress controllers, the control plane, Prometheus and so on), migrations and actions required from users, known limitations and risks. Leave out everything else.
  - The impact on critical components always goes here, even when the `impact` field of a `changes` block repeats it.
  - A changed default value, such as a port, a flag or a parameter, is written as before and after.
  - A changed contract that other components or users rely on, such as API fields, metrics, labels or CRD fields, names who relies on it and what stays compatible.
  - Broader permissions or an exclusion from a protection mechanism are named explicitly, together with why it is safe.
  - When a behavior change can be turned off or the old behavior can be returned, say how.
  - Say where the change works or the problem shows: the settings, editions or scale it depends on.
  - A claim about speed or load comes with a number and says where the number comes from, a measurement or a calculation.
  - How the change was tested goes here as well, in a line or two.
  - When the change has documentation, link to it.
  - When the merge request depends on another one, say so in the first line, together with how to review it, for example that it is stacked on !1046 and is reviewed commit by commit.
  - When the merge request touches several modules, group the bullets by module.
- `## Why do we need it, and what problem does it solve?`: the problem and what it led to, in a few sentences, with a link to the issue when there is one.
- `## Why do we need it in the patch release (if we do)?`: why the change needs a backport, that is who is affected and how serious the problem is, in one or two sentences, or the Russian for "not necessarily" when it does not matter. Remove the section only when the merge request is for a release and not for a patch release.
- `## Checklist`: only the checkboxes of the template, with no text of your own.
- Tables and test results are allowed where the reviewer needs them.
- Subheadings of level `###` and deeper are allowed inside a section where they make it easier to read. In the Russian sections they are in Russian, like the rest of the text. Never use `##` or `#` inside a section: such a heading ends the section, and the CI check reads the text after it as English.

### By kind of change

Each kind of change has a minimum the description must give. A merge request of several kinds gives the minimum of each.

- A fix: the symptom, the cause, what is fixed and who is affected. When known, the release the bug appeared in and the commit that introduced it.
- An update of a version or a dependency: why it is needed (the CVE or another finding, the bug, the feature), what changes for users and what restarts.
- A security fix: the ID of the finding (CVE, Svacer and so on), whether it can be exploited in Deckhouse, and what restarts.
- A new feature: what it gives, how to turn it on and off, its default values and a link to its documentation.

### Length

- The text under the three Russian headings together is at most 5000 characters, and 2000 is the target. Code, tables and test results count. Measure it before you create or update the merge request, for example with `wc -m` on a file with these sections.

### What not to write

- A retelling of the diff: lists of changed files and functions.
- The story of the work: what was tried first, what was found along the way.
- Alternatives considered, unless the choice is not obvious. Then one sentence.
- The same fact in several sections, or again in the `summary` of a `changes` block.
- Generic phrases such as "improves reliability and maintainability".
- Bold text throughout a section.
- Long logs and command output. Use an excerpt of up to about 10 lines or a link.

### Agent attribution

A note that an agent prepared the change, such as a "Generated with" line, may stay in the description, but never inside a `changes` block: the changelog generator copies these blocks into the public changelog.

### Before you submit

Before you create or update the merge request, check the description against these rules: every minimum of its kind is there, nothing from the list of what not to write is, every applicable section is filled in, and the Russian sections fit the limits.

## Test and documentation secrets (gitleaks)

Gitleaks scans the whole repository: tests, testdata, golden files, fixtures, documentation and module examples. Secret-looking example values must not introduce new findings, so every such value must be taken from the list below. These values are allowlisted once in `.gitleaks.toml`.

Rules:

- When writing or reviewing tests, documentation or examples, take any password, token, API key, base64 secret payload, htpasswd or bcrypt hash, JWT, private key or cloud credential from this list. Do not invent new ones, do not copy real ones.
- Pick the value by purpose. If several fit, choose the one whose decoded meaning reads best in context.
- If nothing fits, add a new value to this list and to the last `[[allowlists]]` block in `.gitleaks.toml` in the same change.
- Every new entry in `.gitleaks.toml` gets a comment above it with the date, the MR or issue that added it and the reason, for example `# Added on 2026-10-16 in MR !703: ...`. A whole-file exclusion in `paths` is allowed only for third-party files or fixtures parsed by the code under test, by exact path, and never for files with real secrets.
- The pipeline reads `.gitleaks.toml` from `main`, so a new entry works only after it is merged. Add it in a separate MR before the MR that uses it.
- A value that is not in the list is a gitleaks finding. Do not add per-file or per-value ignores, replace it with a value from the list.
- Inline `gitleaks:allow` comments are ignored by CI (`--ignore-gitleaks-allow`). Never add them.
- Use the encoded form where `Secret.data`, docker config `auth` or similar requires base64. The "Meaning" column shows the decoded text.
- When a hook or code under test needs a value of a specific length or format, extend `dh-test-secret-` with a suffix of that length instead of inventing a new secret. Check what the code actually parses before replacing a value: keys and certificates that are parsed, and names derived by the code under test, must stay as they are.
- Obvious placeholders such as `<YOUR-TOKEN>` or `***` are not findings and are always fine in documentation.

| Purpose | Value | Meaning |
|---|---|---|
| Plain password, token or API key | `dh-test-secret-0123456789abcdef` | |
| Plain value of a required length | `dh-test-secret-` plus any `[0-9A-Za-z]` suffix, e.g. `dh-test-secret-012345678` (24 chars), `dh-test-secret-0123456789abcdef0` (32 chars) | |
| Base64 of any `dh-test-secret-*` value | starts with `ZGgtdGVzdC1zZWNyZXQt`, e.g. `ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4` | `dh-test-secret-012345678` |
| Base64 secret data (`Secret.data`) | `dGVzdC12YWx1ZQ==` | `test-value` |
| Docker config / registry `auth` (base64 `user:password`) | `dXNlcm5hbWU6cGFzc3dvcmQ=` (the value without the trailing `=` is also allowed, for invalid-padding cases) | `username:password` |
| Base64 token | `PFlPVVItR1JBRkFOQUNMT1VELVRPS0VOPg==` | `<YOUR-GRAFANACLOUD-TOKEN>` |
| Base64 htpasswd line | `Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK` | `foo:$apr1$OFG3Xybp$ckL0FHDAkoXYIlH9.cysT0` |
| Base64 bcrypt hash (`User.spec.password`) | `JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H` | `$2y$10$dMXeFQPduGXaS2X1OpgYvOGK/5.Gl6nluOfRHg5iPt8nJXzK8iy.G` |
| Ceph key | `AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==` | |
| GCP service account `private_key_id` | `98sdcj5e8c7asd98j4j3n9csakn` | |
| AWS access key id | `AKIAIOSFODNN7EXAMPLE` | |
| AWS secret access key | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` | |
| JWT | `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkZWNraG91c2UtdGVzdCIsIm5hbWUiOiJ0ZXN0In0.dGVzdC1zaWduYXR1cmUtbm90LXJlYWw` | |

### Private keys

In tests use exactly this body between the markers:

```text
-----BEGIN PRIVATE KEY-----
ZGVja2hvdXNlLXRlc3QtcHJpdmF0ZS1rZXktbm90LWEtcmVhbC1zZWNyZXQtMDEyMzQ1Njc4OWFiY2RlZg==
-----END PRIVATE KEY-----
```

The same body is valid inside `RSA PRIVATE KEY` / `EC PRIVATE KEY` markers and in escaped single-line form (`\n`). If a test needs a cryptographically valid key or certificate (parsing, TLS), generate it at test runtime instead of committing it.

In documentation keep the key elided, exactly in one of these forms:

- Yandex Cloud JSON, single line: `-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n` (with the `\n` written as the two characters backslash and `n`).
- SSH key block:

  ```text
  -----BEGIN OPENSSH PRIVATE KEY-----
  ...
  -----END OPENSSH PRIVATE KEY-----
  ```

- GCP service account JSON: `"private_key": "-----BEGIN PRIVATE KEY-----",` followed by `"client_id": "342975658279248",`.
