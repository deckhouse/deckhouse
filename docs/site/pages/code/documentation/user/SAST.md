---
title: "SAST scanning with Semgrep"
menuTitle: SAST scanning
force_searchable: true
description: Configure the mandated Semgrep SAST scan in Deckhouse Code — rule sets, blocking threshold, path filters, and the scanner image
permalink: en/code/documentation/user/sast.html
lang: en
weight: 91
---

Static Application Security Testing (SAST) in Deckhouse Code looks for vulnerabilities in the source code of a project. The scanner behind it is [Semgrep](https://semgrep.dev/), and the scan is called `sast`.

The scan is *mandated*: it is added to a project's pipeline by a scan execution policy that lives in a separate security policy project, not by the project's own `.gitlab-ci.yml`. What is scanned and what fails the pipeline is decided by the policy — that is, by the security team — and the scanned project cannot override it from its own CI/CD settings.

{% alert level="warning" %}
Security scanning is gated behind the instance-level feature flag `fe_security_scan_policies`, which is **disabled by default**. While it is off, policy pages and scanner integration pages are hidden and no scan jobs are added to pipelines. Ask an instance administrator to enable it.
{% endalert %}

## What the scan does

The `sast` scan adds two jobs to the pipeline, both in the `fe-security-scanner` stage:

| Job | What it does |
|-----|--------------|
| `semgrep_sast_scan` | Runs the scanner over the repository and publishes its raw JSON output as an artifact. |
| `semgrep_sast_junit` | Reads that JSON, builds a JUnit test report and a security report from it, and decides the outcome: the job succeeds when no finding reaches the blocking threshold, and fails otherwise. It runs on a Ruby image rather than the scanner's. |

Both jobs upload their artifacts with `when: always`, so the reports are on disk before the second job may fail — a blocked pipeline never costs you the findings that blocked it.

Findings reach the same places as those of every other policy scanner:

| Report | Where to view |
|--------|---------------|
| Tests (JUnit) | "Tests" tab of the pipeline and the test summary in the merge request |
| Security report (SAST) | Vulnerability report page |
| Findings forwarded to DefectDojo | DefectDojo, when that integration is configured |

## Prerequisites

Before the scan can run, make sure that:

- An instance administrator has enabled the `fe_security_scan_policies` feature flag.
- A security policy project is linked to the project or to a group above it.
- A GitLab Runner with a `docker` executor is available, and it can pull both images the scan uses: the scanner image (see "Where the scanner image comes from" below) and the Ruby image the second job runs on, `ruby:3.3.10-slim` by default, which an installation can redirect with the instance variable `FE_SCANS_REPORT_CONVERTER_IMAGE`.

## Turning the scan on

Add a `sast` action to `.gitlab/security-policies/policy.yml` in the security policy project:

```yaml
scan_execution_policy:
  - name: SAST everywhere
    enabled: true
    rules:
      - type: pipeline
        branches: ["*"]
    actions:
      - scan: sast
```

Then link the policy project to the target project or group in "Settings" → "Security policy". Every pipeline the rules match then gains the two jobs above.

The scan options can also be edited in the policy editor, which renders the form described below.

<!-- TODO(screenshot): the sast action in the policy editor, four blocks collapsed. Navigation theme Neutral, syntax highlighting Light. -->

## What the policy sets

The form for the `sast` action is split into four blocks. Every option is the policy's, not the scanned project's: the values are written into the job by the server while the pipeline is rendered, so the scanned project's own CI/CD variables cannot change them.

### Rules

This block sets which rules the scan runs.

<!-- TODO(screenshot): the "Rules" block with the rule set select open. Navigation theme Neutral, syntax highlighting Light. -->

| Field | What it sets |
|-------|--------------|
| "Rule set" | Where the rules come from. One of four sources, described below. Default: "The set shipped with this product". |
| "Rule set path in the image" | Which of the sets carried inside the scanner image to run (see "Rule sets in the image"). Default: `/rules/lgpl`. The scan stops if the image has no set at this path. |
| "Rules" | The rules themselves, in the scanner's own YAML. Shown when the source is "Rules written in the policy". |
| "Rule file path" | Path to the rule file inside the project the rules are read from. Default: `.semgrep.yml`. |
| "Add to the shipped set" | On by default: your own rules run *alongside* the shipped set. Cleared, they run *instead* of it, and everything the shipped set covered stops being checked. |

The four rule sources differ in who owns the file:

| Source | Where the rules live | Who controls them |
|--------|----------------------|-------------------|
| "The set shipped with this product" | Inside the scanner image, at the path named above | The product release |
| "A file in the security policy project" | The project this policy lives in, on its default branch | The security team |
| "A file in the scanned project" | The repository being scanned, at the commit being built | **The scanned project** |
| "Rules written in the policy" | The policy itself | The policy author |

Three of the four are read by the server and written into the job; only "A file in the scanned project" is read by the job, from its own checkout, at the commit being built. That is also the one source the scanned side writes, so treat it accordingly: a commit that rewrites that file rewrites the rules of the scan that checks it. Leaving "Add to the shipped set" on keeps a baseline in place even when the project's file is emptied.

A rule file the policy named but the scan could not be given is an error, not a fallback: the job prints why and stops, rather than scanning with the shipped set and reporting success.

### Blocking

This block sets what a finding does to the pipeline.

| Field | What it sets |
|-------|--------------|
| "Blocking threshold" | What fails the pipeline. Findings below the threshold are still reported and still reach the vulnerability report — the threshold decides the pipeline's outcome and nothing else. Default: "High and above". |

The threshold has four values:

| Value | What fails the pipeline |
|-------|--------------------------|
| "High and above" (default) | A high finding. Medium and info are reported. |
| "Medium and above" | Medium and high. Info is reported. |
| "Any finding" | Any finding, whatever its level. |
| "Report only, block nothing" | Nothing. Findings are still reported and still reach the vulnerability report and DefectDojo. |

The list is short on purpose. Semgrep has three severity levels — ERROR, WARNING and INFO — which the scan reports as high, medium and info. There is no critical and no low, so a threshold named after either would be a threshold no finding can ever reach: "critical" would block nothing while looking strict, and "low" would be a duplicate of "medium". "Report only, block nothing" exists so that not blocking is a visible choice in the policy rather than a side effect of picking an unreachable level.

### Paths

This block sets what the scan looks at, and what it leaves out.

| Field | What it sets |
|-------|--------------|
| "Path filters" | Where the lists of paths to leave out come from. Default: "No filters". |
| "Filter file path" | Path to the filter file inside the project the filters are read from. Default: `.semgrepignore`. |
| "Skip these paths" | One path or pattern per line: third-party code, fixtures, generated files. |
| "Look only at these paths" | One path or pattern per line. Everything else is left out of the scan entirely. |
| "Honour the repository's .gitignore" | Off by default. |

"No filters" does not mean nothing is skipped: the job always adds its own built-in exclusions — `node_modules/`, `vendor/`, `dist/`, `build/`, `.venv/` and `.git/` — to whatever the policy set, so a filter file of your own never silently drags dependency trees back into the scan.

Filters have the same four-way choice of source as rules:

| Source | Where the list lives | Who controls it |
|--------|----------------------|-----------------|
| "No filters" | — | — |
| "A file in the security policy project" | Beside the policy, applied to every project it covers | The security team |
| "A file in the scanned project" | The repository being scanned | **The scanned project** |
| "Lists written in the policy" | The two lists in the form | The policy author |

By default the job sets aside any `.semgrepignore` it finds in the scanned repository — including nested ones in subdirectories — before the scan starts. Picking "A file in the scanned project" is what turns that off, and it is worth being deliberate about:

{% alert level="warning" %}
"A file in the scanned project" and "Honour the repository's .gitignore" both hand the scanned project the say over what the mandated scan examines. A commit in that project can then narrow the scan that checks it, and the narrowing is invisible in the policy. Both are legitimate delegations — a security team may well decide each project knows its own third-party directories best — but they are delegations, and the policy is where that decision is recorded. Neither is on by default.
{% endalert %}

Two more things about the two lists:

- "Look only at these paths" is stronger than "Skip these paths". An exclusion removes what it names; an inclusion removes everything else. A single `src/**` quietly takes `lib`, `config` and your migrations out of the scan.
- They are applied in that order — inclusions first, then exclusions — so an exclusion still applies inside what an inclusion let through.

A pattern that matches nothing is not a syntax error and the form cannot catch it. What catches it is the job: it prints how many files the scanner actually read, and fails when that number is zero.

### Advanced

This block sets which levels of rule run, and the flags this form does not name.

| Field | What it sets |
|-------|--------------|
| "Rule levels" | Which rules run at all: "High" (ERROR), "Medium" (WARNING), "Info" (INFO). All three are on by default. |
| "Additional arguments" | Flags passed to the scan as written, for what this form does not name. |

"Rule levels" and "Blocking threshold" look alike and do different things:

| Field | Effect | A finding below the level |
|-------|--------|---------------------------|
| "Rule levels" | Which rules are **evaluated** | Does not exist: the rule never ran |
| "Blocking threshold" | Which findings **fail the pipeline** | Is in the report, but does not block |

So leaving a level out removes its findings from the report as well, not merely from blocking — the vulnerability report and DefectDojo both get smaller, and they get smaller silently. A blocking threshold set below the lowest level that runs cannot fire at all; the form says so when that happens.

"Additional arguments" is for tuning the runner's resources — timeouts, memory limits, maximum file size — not for changing what the scan is. Flags that redirect or silence the report are refused, as are the flags this form already owns: output and report formats (`--json`, `--sarif`, `--junit-xml`, `--gitlab-sast`, `--gitlab-secrets`, `--output`, `--text` and their paired `*-output` forms), `--config`, `--metrics` and `--baseline-commit`.

## Rule sets in the image

By default the scan runs the rule set carried inside the scanner image, at `/rules/lgpl`. The image carries more than one set, and "Rule set path in the image" chooses between them:

| Path in the image | Rules | Languages |
|-------------------|-------|-----------|
| `/rules/lgpl` (default) | 152 | `generic`, `java`, `javascript`, `typescript`, `kotlin`, `swift` |
| `/rules/lgpl-cc` | 97 | `generic`, `java`, `javascript`, `typescript`, `php`, `python`, `ruby`, `yaml` |
| `/rules` | 338 | `c`, `c++`, `c#`, `go`, `java`, `javascript`, `typescript`, `python`, `scala` |

These figures were measured in `registry.gitlab.com/security-products/semgrep:6.25.0` and describe that image; a newer image may carry a different set. Each directory in the image carries its own license file, and which set to run is your choice to make.

{% alert level="warning" %}
The sets are not nested: `/rules/lgpl-cc` adds python, ruby and php but loses kotlin and swift, and `/rules` covers neither kotlin, nor swift, nor php, nor ruby. The default set therefore does not cover Python, Go, Ruby, C#, PHP or C/C++. A repository written in one of those, scanned at the default path, finishes successfully with no findings — which is the scan honestly saying that no rule applied to it, not a broken scan. It also means that repository has no SAST coverage at all: choose `/rules/lgpl-cc` or `/rules` for it explicitly rather than relying on the default.
{% endalert %}

Rules of your own are the other half of the answer. They are added through the "Rule set" field and, with "Add to the shipped set" left on, run alongside whichever set the image provides.

## Where the scanner image comes from

The image the scan runs in is resolved by the server once, before the job starts, and written into the job as a literal value. It never becomes a CI/CD variable, so the scanned project's own settings cannot point the scan at a different scanner.

The sources are listed from weakest to strongest — each one, when set, overrides the one above it:

1. The version shipped with this product. This is what runs when nothing else is set.
1. The instance environment variable `FE_SCANS_SEMGREP_IMAGE` — the decision of the administrator of the whole installation.
1. The Semgrep integration on the group that owns the policy, or on a group above it (see below).
1. The version named in the policy itself, in the "Scanner version" field.

There is exactly one exception: when the integration names a full image reference rather than a registry — with a tag or a digest — that reference is used as it is, and the version from the policy no longer has anything to replace.

The scanned project's own integration record is ignored. The image decides what "the scan ran" even means, and the side being checked does not answer that question.

### The Semgrep integration

The integration holds one thing: where the scanner image is read from. It is set on a **group**, so an installation says it once instead of repeating it in every policy; a project sees the integration with no fields, which is how a reader learns the scan is configured above it.

Open "Settings" → "Integrations" → "Semgrep" on the group and fill in either field — both are optional:

| Field | What it sets |
|-------|--------------|
| "Registry" | The registry the scanner image is mirrored in, without a tag. For example, `registry.example.com` or `registry.example.com/mirror`. The scan reads the image from here instead of from the one shipped with this product, at the version the policy asks for. |
| "Prepared image" | One image, named in full and with a tag or a digest, for an installation that builds its own. For example, `registry.example.com/ci-images/semgrep:6.25.0`. It is used exactly as written, so the version in a policy no longer applies. Takes precedence over "Registry". |

With both fields empty, the scan runs the image shipped with this product. Nothing about what the scan looks for is set here — that lives in the policy.

<!-- TODO(screenshot): the Semgrep integration form on a group, both fields empty. Navigation theme Neutral, syntax highlighting Light. -->

### The version shipped with this product

The default is a specific version, not a floating tag: this release ships `registry.gitlab.com/security-products/semgrep:6.25.0`. One tag pins both halves of the scanner at once — the engine and the rule sets carried inside the image — because both live in the same image.

The "Scanner version" field in the policy takes either form of pin:

- A version number, such as `6.25.0`, which a person can read and compare.
- A digest, such as `sha256:aafbcaff…`, which names one build and cannot be moved to another.

Floating tags (`latest`, `stable`, a tag that is a major number on its own, such as `6`) and references that carry a registry address are refused: the registry is the integration's to name, and the policy names only the version.

The version shipped with a release is reviewed when Deckhouse Code is updated, so it moves forward with the product rather than drifting.

### Installations with no route to the internet

The scan uses two images, and an installation with no route to the internet has to provide both:

| Image | Default | How to redirect it |
|-------|---------|--------------------|
| Scanner | `registry.gitlab.com/security-products/semgrep:6.25.0` | The "Registry" or "Prepared image" field of the Semgrep integration |
| Report converter | `ruby:3.3.10-slim`, from Docker Hub | The instance variable `FE_SCANS_REPORT_CONVERTER_IMAGE` |

Mirror them into your own registry first, exactly as GitLab's own offline documentation prescribes for its analyzer images, and then point the corresponding setting at that registry. Deckhouse Code names both images; it ships neither.

{% alert level="info" %}
The GitLab dependency proxy does not cover the scanner image. It is a pull-through cache for images stored on Docker Hub, and the scanner image is published on `registry.gitlab.com`, so the proxy has nothing to offer it — the Ruby converter image is the only one of the two it can cache. For an installation that wants to stop pulling the scanner image from outside on every run, with or without a route to the internet, the answer is the same one: mirror it into your own registry and point the "Registry" field at it.
{% endalert %}

## Semgrep's paid features

Everything described on this page runs on the open version of the scanner. Several Semgrep capabilities are not part of it:

| Capability | In the open version | What Deckhouse Code offers instead |
|------------|---------------------|-------------------------------------|
| Cross-file data-flow analysis | No, within a single file only | Nothing; the limitation stands |
| Pro rules | No | Rules of your own, through the "Rule set" field |
| Dependency analysis (Supply Chain) | No | The CodeScoring integration, or the `dependency_scanning` scan |
| Validity checking of discovered secrets | No | Nothing |

{% alert level="warning" %}
The paid capabilities work through the vendor's cloud. Turning them on means the scan uploads its findings — and the code context around them — to Semgrep's servers, outside your installation. In an installation with no route to the internet they are unavailable for a plainer reason: there is nothing to reach them through.
{% endalert %}

The mandated scan sends nothing anywhere. It runs `semgrep scan` with `--metrics=off` and `--disable-version-check`, not `semgrep ci`, so neither findings nor usage telemetry leave the job. Before the scanner is even located, the job clears every `SEMGREP_*` and `PYTHON*` variable it inherited: two of them, `SEMGREP_RULES` and `SEMGREP_BASELINE_COMMIT`, are documented equivalents of `--config` and `--baseline-commit`, and left in place they would let the scanned project's CI/CD settings replace the rule set or the comparison base of the scan that checks it.

## Troubleshooting

The sections below list common problems and what to check for each.

### The scan does not appear in the pipeline

Check the following:

- The `fe_security_scan_policies` feature flag is enabled on the instance.
- The security policy project is linked to the project, and `policy.yml` contains a `- scan: sast` action.
- The policy's rules match the pipeline (branch, pipeline type).
- A GitLab Runner with a `docker` executor is available.

### The job fails with "no rule set at … in this image"

The path in "Rule set path in the image" does not exist in the image the scan resolved. Check the path against the table above, and check which image is actually in use — a "Prepared image" set on the integration may carry a different layout.

### The job fails with "semgrep read no files at all"

The scan examined nothing, so it refuses to report success. The usual causes are a "Look only at these paths" pattern that matches nothing, filters that removed the whole repository, or a checkout with nothing in it. The job's log prints how many files it read and how many the filters removed.

### The scan succeeds but finds nothing

Most often the rule set does not cover the language of the repository — see the warning in "Rule sets in the image". Check the job log: it prints the rule set path in use and the number of rules loaded. Narrowing "Rule levels" has the same effect for a different reason: a level that is not switched on never runs, so its findings are absent from the report rather than merely not blocking.
