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

"""Report reproducibility rebuild timing from GitLab pipelines.

The script searches scheduled pipelines from a configurable look-back window,
collects scheduled pipeline IDs from pipelines list, then measures successful
job duration for the exact job name "build-fe:reproducibility".

Only Python stdlib is used.

Environment variables:
  WINDOW_DAYS            Look-back window in days (default: 7)

  CI_API_V4_URL          GitLab API base URL (preferred; e.g. https://gitlab.com/api/v4)
  GITLAB_API_V4_URL      Alternative API base URL override

  CI_PROJECT_ID          Numeric project id (preferred)
  CI_PROJECT_PATH        Path project id fallback (group/project)
  GITLAB_PROJECT         Explicit project id/path override

  GITLAB_TOKEN           Private/access token for PRIVATE-TOKEN auth
  PRIVATE_TOKEN          Alternative private/access token
  CI_JOB_TOKEN           Job token for JOB-TOKEN auth fallback

  TARGET_JOB_NAME        Exact job name to measure (default: build-fe:reproducibility)

  GITLAB_DOTENV_FILE     Dotenv output path for downstream jobs
  DOTENV_FILE            Alternative dotenv output path fallback
  TIMING_REPORT_SHELL_FILE
                          Shell-sourceable state output path for same-job after_script

  GITLAB_STEP_SUMMARY    Markdown summary output path
  CI_JOB_SUMMARY         GitLab native summary file path fallback

Dotenv output variables:
  run_count, avg_seconds, avg_build_time
  RUN_COUNT, AVG_SECONDS, AVG_BUILD_TIME
  pipeline_count, PIPELINE_COUNT

Shell state output variables:
  RUN_COUNT, AVG_SECONDS, AVG_BUILD_TIME, PIPELINE_COUNT
"""

from __future__ import annotations

import json
import os
import shlex
import sys
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Any


def _parse_ts(value: str | None) -> datetime | None:
    """Parse GitLab ISO timestamps to UTC, returning None for empty/invalid values."""
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(
            timezone.utc
        )
    except ValueError:
        return None


def _fmt_duration(total_seconds: float) -> str:
    """Format seconds as a compact human-readable duration used in reports."""
    seconds = int(round(total_seconds))
    hours, rem = divmod(seconds, 3600)
    minutes, sec = divmod(rem, 60)
    parts: list[str] = []
    if hours:
        parts.append(f"{hours}h")
    if hours or minutes:
        parts.append(f"{minutes}m")
    parts.append(f"{sec}s")
    return " ".join(parts)


def _to_int(name: str, raw: str, default: int) -> int:
    """Convert environment value to int with a field-specific validation error."""
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        raise ValueError(f"{name} must be integer, got: {raw!r}")


@dataclass
class PipelineRow:
    """Rendered report row for a single measured pipeline job."""

    pipeline_id: int
    date: str
    duration_seconds: float
    duration_human: str
    pipeline_url: str
    job_url: str


class GitLabAPIError(RuntimeError):
    """Structured GitLab HTTP error with status, URL and API message."""

    def __init__(self, status_code: int, url: str, message: str) -> None:
        super().__init__(f"GitLab API error {status_code} for {url}: {message}")
        self.status_code = status_code
        self.url = url
        self.message = message


class GitLabAPI:
    """Minimal GitLab API client with token auth and pagination helpers."""

    def __init__(self, base_url: str, token: str, token_kind: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.token_kind = token_kind

    def _request(self, url: str) -> tuple[Any, dict[str, str]]:
        """Perform one JSON request and normalize transport/API failures."""
        req = urllib.request.Request(url)
        req.add_header("Accept", "application/json")
        if self.token_kind == "private":
            req.add_header("PRIVATE-TOKEN", self.token)
        else:
            req.add_header("JOB-TOKEN", self.token)

        try:
            with urllib.request.urlopen(req) as resp:
                body_raw = resp.read().decode("utf-8")
                body = json.loads(body_raw) if body_raw else None
                headers = {k: v for k, v in resp.headers.items()}
                return body, headers
        except urllib.error.HTTPError as err:
            msg = err.read().decode("utf-8", errors="replace")
            try:
                payload = json.loads(msg)
                if isinstance(payload, dict):
                    msg = payload.get("message") or payload.get("error") or msg
            except json.JSONDecodeError:
                pass
            raise GitLabAPIError(err.code, url, msg) from err
        except urllib.error.URLError as err:
            raise RuntimeError(f"GitLab API connection error for {url}: {err}") from err

    def get(
        self, path: str, params: dict[str, str] | None = None
    ) -> tuple[Any, dict[str, str]]:
        """Execute a GET request for API path with encoded query parameters."""
        query = urllib.parse.urlencode(params or {}, doseq=True)
        url = f"{self.base_url}{path}"
        if query:
            url = f"{url}?{query}"
        return self._request(url)

    def paginate(
        self, path: str, params: dict[str, str] | None = None
    ) -> list[dict[str, Any]]:
        """Fetch all pages using GitLab X-Next-Page headers."""
        page = 1
        all_items: list[dict[str, Any]] = []
        params_base = dict(params or {})

        while True:
            params_page = dict(params_base)
            params_page["page"] = str(page)
            params_page.setdefault("per_page", "100")

            body, headers = self.get(path, params_page)
            if not isinstance(body, list):
                raise RuntimeError(
                    f"Expected list response for {path}, got {type(body).__name__}"
                )

            all_items.extend(body)
            next_page = (headers.get("X-Next-Page") or "").strip()
            if not next_page:
                break
            try:
                page = int(next_page)
            except ValueError as err:
                raise RuntimeError(
                    f"Invalid X-Next-Page header for {path}: {next_page!r}"
                ) from err

        return all_items


def _resolve_auth() -> tuple[str, str]:
    """Resolve auth token and header type, preferring private token over job token."""
    private_token = os.environ.get("GITLAB_TOKEN") or os.environ.get("PRIVATE_TOKEN")
    if private_token:
        return private_token, "private"

    job_token = os.environ.get("CI_JOB_TOKEN")
    if job_token:
        return job_token, "job"

    raise RuntimeError(
        "Authentication is required: set GITLAB_TOKEN/PRIVATE_TOKEN or CI_JOB_TOKEN"
    )


def _resolve_api_base() -> str:
    """Resolve GitLab API v4 base URL from CI environment variables."""
    api_base = os.environ.get("GITLAB_API_V4_URL") or os.environ.get("CI_API_V4_URL")
    if api_base:
        return api_base.rstrip("/")

    server = (os.environ.get("CI_SERVER_URL") or "https://gitlab.com").rstrip("/")
    return f"{server}/api/v4"


def _resolve_project() -> str:
    """Resolve and URL-encode project identifier accepted by GitLab endpoints."""
    project = (
        os.environ.get("GITLAB_PROJECT")
        or os.environ.get("CI_PROJECT_ID")
        or os.environ.get("CI_PROJECT_PATH")
    )
    if not project:
        raise RuntimeError(
            "Project is required: set GITLAB_PROJECT or CI_PROJECT_ID/CI_PROJECT_PATH"
        )
    return urllib.parse.quote(project, safe="")


def _measure_job_duration(job: dict[str, Any]) -> float | None:
    """Measure duration from timestamps, falling back to numeric duration field."""
    started_at = _parse_ts(job.get("started_at"))
    finished_at = _parse_ts(job.get("finished_at"))
    if started_at and finished_at:
        seconds = (finished_at - started_at).total_seconds()
        if seconds >= 0:
            return seconds

    duration = job.get("duration")
    if isinstance(duration, (int, float)) and duration >= 0:
        return float(duration)

    return None


def collect_rows(
    api: GitLabAPI,
    project_q: str,
    since: datetime,
    target_job_name: str,
) -> tuple[list[PipelineRow], int]:
    """Collect measured rows and count pipelines that contain the exact target job."""
    params = {
        "source": "schedule",
        "order_by": "id",
        "sort": "desc",
        "per_page": "100",
    }
    pipelines = api.paginate(f"/projects/{project_q}/pipelines", params)

    print(f"Found {len(pipelines)} scheduled pipeline(s) in API response")

    rows: list[PipelineRow] = []
    target_pipeline_count = 0
    for p in pipelines:
        if not isinstance(p, dict):
            continue

        created_at = _parse_ts(p.get("created_at"))
        if not created_at or created_at < since:
            continue

        pipeline_id_raw = p.get("id")
        try:
            pipeline_id = int(pipeline_id_raw)
        except (TypeError, ValueError):
            print(f"Skipping pipeline with invalid id: {pipeline_id_raw!r}")
            continue

        jobs = api.paginate(
            f"/projects/{project_q}/pipelines/{pipeline_id}/jobs",
            {"include_retried": "true", "per_page": "100"},
        )

        target_jobs = [
            j for j in jobs if isinstance(j, dict) and j.get("name") == target_job_name
        ]
        if not target_jobs:
            continue

        target_pipeline_count += 1

        candidate_jobs = [j for j in target_jobs if j.get("status") == "success"]
        if not candidate_jobs:
            print(
                f"pipeline={pipeline_id}: no successful '{target_job_name}' job found, skipping"
            )
            continue

        candidate_with_ids: list[tuple[int, dict[str, Any]]] = []
        for job_candidate in candidate_jobs:
            try:
                job_id = int(job_candidate.get("id"))
            except (TypeError, ValueError):
                continue
            candidate_with_ids.append((job_id, job_candidate))

        if not candidate_with_ids:
            print(
                f"pipeline={pipeline_id}: no valid successful '{target_job_name}' job id found, skipping"
            )
            continue

        _, job = max(candidate_with_ids, key=lambda item: item[0])

        duration = _measure_job_duration(job)
        if duration is None:
            print(
                f"pipeline={pipeline_id}: no valid duration for '{target_job_name}', skipping"
            )
            continue

        pipeline_url = str(p.get("web_url") or "")
        job_url = str(job.get("web_url") or "")
        row = PipelineRow(
            pipeline_id=pipeline_id,
            date=created_at.date().isoformat(),
            duration_seconds=duration,
            duration_human=_fmt_duration(duration),
            pipeline_url=pipeline_url,
            job_url=job_url,
        )
        rows.append(row)

    rows.sort(key=lambda r: (r.date, r.pipeline_id))
    return rows, target_pipeline_count


def write_dotenv(
    path: str,
    run_count: int,
    pipeline_count: int,
    avg_seconds: int,
    avg_build_time: str,
) -> None:
    """Append metrics to GitLab dotenv output for downstream jobs."""
    if not path:
        return

    with open(path, "a", encoding="utf-8") as fp:
        fp.write(f"run_count={run_count}\n")
        fp.write(f"avg_seconds={avg_seconds}\n")
        fp.write(f"avg_build_time={avg_build_time}\n")
        fp.write(f"pipeline_count={pipeline_count}\n")

        fp.write(f"RUN_COUNT={run_count}\n")
        fp.write(f"AVG_SECONDS={avg_seconds}\n")
        fp.write(f"AVG_BUILD_TIME={avg_build_time}\n")
        fp.write(f"PIPELINE_COUNT={pipeline_count}\n")


def write_shell_env(
    path: str,
    run_count: int,
    pipeline_count: int,
    avg_seconds: int,
    avg_build_time: str,
) -> None:
    """Write shell-sourceable state for same-job after_script."""
    if not path:
        return

    with open(path, "w", encoding="utf-8") as fp:
        fp.write(f"RUN_COUNT={run_count}\n")
        fp.write(f"AVG_SECONDS={avg_seconds}\n")
        fp.write(f"AVG_BUILD_TIME={shlex.quote(avg_build_time)}\n")
        fp.write(f"PIPELINE_COUNT={pipeline_count}\n")


def write_summary(
    path: str,
    window_days: int,
    rows: list[PipelineRow],
    avg_build_time: str,
    target_pipeline_count: int,
) -> None:
    """Append markdown summary with totals, average duration and per-pipeline rows."""
    if not path:
        return

    with open(path, "a", encoding="utf-8") as fp:
        fp.write(f"## Build reproducibility timing (last {window_days} days)\n\n")
        fp.write(
            f"Scheduled pipelines with exact target job: {target_pipeline_count}\n"
        )
        fp.write(f"Successful measured jobs: {len(rows)}\n")
        fp.write(f"Average rebuild time: {avg_build_time}\n\n")

        if rows:
            fp.write("| Date | Pipeline | Rebuild time | Job |\n")
            fp.write("|---|---:|---:|---|\n")
            for r in rows:
                pipeline_link = (
                    f"[#{r.pipeline_id}]({r.pipeline_url})"
                    if r.pipeline_url
                    else str(r.pipeline_id)
                )
                job_link = f"[link]({r.job_url})" if r.job_url else ""
                fp.write(
                    f"| {r.date} | {pipeline_link} | {r.duration_human} | {job_link} |\n"
                )


def main() -> int:
    """Entry point: resolve config, collect timings, print report and write outputs."""
    try:
        days = _to_int("WINDOW_DAYS", os.environ.get("WINDOW_DAYS", "7"), 7)
        if days < 1:
            raise ValueError("WINDOW_DAYS must be >= 1")

        target_job_name = os.environ.get("TARGET_JOB_NAME", "build-fe:reproducibility")

        api_base = _resolve_api_base()
        project_q = _resolve_project()
        token, token_kind = _resolve_auth()

        dotenv_path = os.environ.get("GITLAB_DOTENV_FILE") or os.environ.get(
            "DOTENV_FILE", ""
        )
        shell_env_path = os.environ.get("TIMING_REPORT_SHELL_FILE", "")
        summary_path = os.environ.get("GITLAB_STEP_SUMMARY") or os.environ.get(
            "CI_JOB_SUMMARY", ""
        )

        since = datetime.now(timezone.utc) - timedelta(days=days)
        print(
            f"Collecting scheduled pipelines since {since.isoformat()} "
            f"for job='{target_job_name}'"
        )

        api = GitLabAPI(api_base, token, token_kind)
        rows, target_pipeline_count = collect_rows(
            api,
            project_q,
            since,
            target_job_name,
        )

        print()
        print("| Date | Pipeline ID | Build job duration | Job |")
        print("|---|---:|---:|---|")
        for r in rows:
            print(
                f"| {r.date} | {r.pipeline_id} | {r.duration_human} | "
                f" {r.job_url} |"
            )
        print()

        if rows:
            avg_seconds_float = sum(r.duration_seconds for r in rows) / len(rows)
            avg_seconds = int(round(avg_seconds_float))
            avg_build_time = _fmt_duration(avg_seconds_float)
            print(
                f"Average rebuild time over last {days} day(s): {avg_build_time} "
                f"({len(rows)} successful run(s) in {target_pipeline_count} scheduled pipeline(s) with target job)"
            )
        else:
            avg_seconds = 0
            avg_build_time = "n/a"
            print(
                f"No successful '{target_job_name}' jobs found in {target_pipeline_count} "
                f"scheduled pipeline(s) with target job over last {days} day(s)."
            )

        write_dotenv(
            dotenv_path,
            run_count=len(rows),
            pipeline_count=target_pipeline_count,
            avg_seconds=avg_seconds,
            avg_build_time=avg_build_time,
        )
        write_shell_env(
            shell_env_path,
            run_count=len(rows),
            pipeline_count=target_pipeline_count,
            avg_seconds=avg_seconds,
            avg_build_time=avg_build_time,
        )
        write_summary(summary_path, days, rows, avg_build_time, target_pipeline_count)

        return 0
    except Exception as err:  # pylint: disable=broad-exception-caught
        print(f"ERROR: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
