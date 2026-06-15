#!/usr/bin/env python3

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Report rebuild timings of scheduled build-reproducibility-check runs.

Queries the GitHub Actions API for SCHEDULED runs of the reproducibility check
workflow within a look-back window, measures the duration of the successful
'Build FE' rebuild job in each run, prints a per-run table and the average
rebuild time to the log, and exposes the results (run_count, avg_seconds,
avg_build_time) via GITHUB_OUTPUT for downstream CI steps.

Only stdlib is used (urllib) so the script needs no extra dependencies.

Environment variables (all optional ones have defaults):
    GITHUB_TOKEN        - token for GitHub API calls (required)
    GITHUB_REPOSITORY   - 'owner/repo' (required; set by GitHub Actions)
    GITHUB_API_URL      - GitHub API base (default https://api.github.com)
    WORKFLOW_FILE       - workflow file to inspect (default reproducibility-check.yml)
    BUILD_JOB_PREFIX    - build job name prefix to match (default 'Build FE')
    WINDOW_DAYS         - look-back window in days (default 7)
    GITHUB_OUTPUT       - if set, emit run_count/avg_seconds/avg_build_time
    GITHUB_STEP_SUMMARY - if set, write a markdown summary table
"""

import json
import os
import sys
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone

_GITHUB_TS = "%Y-%m-%dT%H:%M:%SZ"


def api_get(url: str, token: str):
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    with urllib.request.urlopen(req) as resp:
        body = json.loads(resp.read().decode("utf-8"))
        link = resp.headers.get("Link", "")
    return body, link


def next_link(link_header: str):
    for part in link_header.split(","):
        section = part.split(";")
        if len(section) < 2:
            continue
        url = section[0].strip().lstrip("<").rstrip(">")
        if any(p.strip() == 'rel="next"' for p in section[1:]):
            return url
    return None


def paginate(url: str, token: str, list_key: str) -> list:
    items = []
    while url:
        body, link = api_get(url, token)
        items.extend(body.get(list_key, []) if isinstance(body, dict) else body)
        url = next_link(link)
    return items


def fmt_duration(total_seconds: float) -> str:
    s = int(round(total_seconds))
    h, rem = divmod(s, 3600)
    m, sec = divmod(rem, 60)
    parts = []
    if h:
        parts.append(f"{h}h")
    if h or m:
        parts.append(f"{m}m")
    parts.append(f"{sec}s")
    return " ".join(parts)


def parse_ts(value: str) -> datetime:
    return datetime.strptime(value, _GITHUB_TS).replace(tzinfo=timezone.utc)


def collect_rows(owner, repo, api_url, token, workflow_file, build_prefix, since):
    """Return (rows, durations) for successful build jobs created after `since`."""
    since_date = since.date().isoformat()
    runs_url = (
        f"{api_url}/repos/{owner}/{repo}/actions/workflows/"
        f"{urllib.parse.quote(workflow_file)}/runs"
        f"?event=schedule&created=%3E%3D{since_date}&per_page=100"
    )
    runs = paginate(runs_url, token, "workflow_runs")
    print(f"Found {len(runs)} scheduled run(s) in the API window")

    rows = []
    durations = []
    for run in runs:
        created_at = run.get("created_at", "")
        # The 'created' filter is date-granular, so re-check the exact window.
        if not created_at or parse_ts(created_at) < since:
            continue

        jobs_url = f"{api_url}/repos/{owner}/{repo}/actions/runs/{run['id']}/jobs?per_page=100"
        jobs = paginate(jobs_url, token, "jobs")
        build_job = next(
            (j for j in jobs if (j.get("name") or "").startswith(build_prefix)), None
        )
        if build_job is None:
            print(f"run={run['id']}: no '{build_prefix}' job found, skipping")
            continue
        if build_job.get("conclusion") != "success":
            print(
                f"run={run['id']}: build job conclusion="
                f"'{build_job.get('conclusion')}', not success, skipping"
            )
            continue
        started, completed = build_job.get("started_at"), build_job.get("completed_at")
        if not started or not completed:
            print(f"run={run['id']}: build job has no start/finish time, skipping")
            continue

        dur = (parse_ts(completed) - parse_ts(started)).total_seconds()
        durations.append(dur)
        rows.append(
            {
                "date": created_at[:10],
                "dur_human": fmt_duration(dur),
                "url": build_job.get("html_url", ""),
            }
        )

    rows.sort(key=lambda r: r["date"])
    return rows, durations


def write_summary(path, days, rows, durations, avg_build_time):
    if not path:
        return
    with open(path, "a", encoding="utf-8") as fp:
        fp.write(f"## Build reproducibility — rebuild timing (last {days} days)\n\n")
        fp.write(f"Successful scheduled build jobs: {len(durations)}\n\n")
        fp.write(f"Average rebuild time: {avg_build_time}\n\n")
        if rows:
            fp.write("| Date | Rebuild time | Job |\n")
            fp.write("|---|---|---|\n")
            for r in rows:
                fp.write(f"| {r['date']} | {r['dur_human']} | [link]({r['url']}) |\n")


def write_outputs(path, run_count, avg_seconds, avg_build_time):
    if not path:
        return
    with open(path, "a", encoding="utf-8") as fp:
        fp.write(f"run_count={run_count}\n")
        fp.write(f"avg_seconds={avg_seconds}\n")
        fp.write(f"avg_build_time={avg_build_time}\n")


def main() -> int:
    token = os.environ.get("GITHUB_TOKEN")
    repository = os.environ.get("GITHUB_REPOSITORY", "")
    if not token or "/" not in repository:
        print(
            "ERROR: GITHUB_TOKEN and GITHUB_REPOSITORY (owner/repo) are required",
            file=sys.stderr,
        )
        return 1
    owner, repo = repository.split("/", 1)

    api_url = os.environ.get("GITHUB_API_URL", "https://api.github.com").rstrip("/")
    workflow_file = os.environ.get("WORKFLOW_FILE", "reproducibility-check.yml")
    build_prefix = os.environ.get("BUILD_JOB_PREFIX", "Build FE")
    days = int(os.environ.get("WINDOW_DAYS", "7"))

    since = datetime.now(timezone.utc) - timedelta(days=days)
    print(f"Collecting scheduled '{workflow_file}' runs created since {since.isoformat()}")

    rows, durations = collect_rows(
        owner, repo, api_url, token, workflow_file, build_prefix, since
    )

    print()
    print("| Date | Build job duration | Job link |")
    print("|---|---|---|")
    for r in rows:
        print(f"| {r['date']} | {r['dur_human']} | {r['url']} |")
    print()

    if durations:
        avg_seconds = sum(durations) / len(durations)
        avg_build_time = fmt_duration(avg_seconds)
        print(
            f"Average rebuild time over the last {days} days "
            f"({len(durations)} successful run(s)): {avg_build_time}"
        )
    else:
        avg_seconds = 0.0
        avg_build_time = "n/a"
        print(f"No successful build jobs found over the last {days} days.")

    write_summary(os.environ.get("GITHUB_STEP_SUMMARY"), days, rows, durations, avg_build_time)
    write_outputs(
        os.environ.get("GITHUB_OUTPUT"),
        len(durations),
        int(round(avg_seconds)),
        avg_build_time,
    )

    return 0


if __name__ == "__main__":
    sys.exit(main())
