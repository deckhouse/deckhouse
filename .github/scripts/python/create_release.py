import os
import requests


GITHUB_API_URL = os.getenv("GITHUB_API_URL")
REPO_OWNER = os.getenv("REPO_OWNER")
REPO_NAME = os.getenv("REPO_NAME")
ACCESS_TOKEN = os.getenv("ACCESS_TOKEN")
TAG_NAME = os.getenv("TAG_NAME")
RELEASE_BRANCH = os.getenv("RELEASE_BRANCH")
RELEASE_NAME = os.getenv("RELEASE_NAME")
RELEASE_BODY = os.getenv("RELEASE_BODY")
MILESTONE_TITLE = os.getenv("MILESTONE_TITLE")
DRAFT = False
PRERELEASE = True

def check_milestone_and_issue():
    headers = {
        "Authorization": f"Bearer {ACCESS_TOKEN}",
        "Accept": "application/vnd.github.v3+json"
    }
    issue_url = f"{GITHUB_API_URL}/repos/{REPO_OWNER}/{REPO_NAME}/issues?labels=issue/release"
    issue_response = requests.get(issue_url, headers=headers)
    
    if issue_response.status_code != 200:
        raise Exception(f"ERROR: Failed to retrieve the issue for the release '{RELEASE_NAME}'\nResponse code: {issue_response.status_code}\nError message: {issue_response.json()}")
    issues = issue_response.json()
    issue_exists = False
    for issue in issues:
        if "milestone" in issue and issue['milestone']['title'] == TAG_NAME:
            issue_exists = True
            if issue['title'] != f"Release {TAG_NAME}":
                raise Exception(f"ERROR: Incorrect issue name for the release: {RELEASE_NAME}\nIssue url: {issue['html_url']}")
            if issue['state'] != "open":
                raise Exception(f"ERROR: Issue for the release: {RELEASE_NAME} is closed\nIssue url: {issue['html_url']}")
            if issue['milestone']['state'] != "open":
                raise Exception(f"ERROR: Milestone for the release {RELEASE_NAME} is closed\n Milestone url: {issue['milestone']['html_url']}")
            break
    if not issue_exists:
        raise Exception(f"ERROR: Issue not found for release: {RELEASE_NAME}")

def create_github_release():
    headers = {
        "Authorization": f"Bearer {ACCESS_TOKEN}",
        "Accept": "application/vnd.github.v3+json"
    }

    url = f"{GITHUB_API_URL}/repos/{REPO_OWNER}/{REPO_NAME}/releases"
    data = {
        "tag_name": TAG_NAME,
        "target_commitish": RELEASE_BRANCH,
        "name": RELEASE_NAME,
        "body": RELEASE_BODY,
        "draft": DRAFT,
        "prerelease": PRERELEASE
    }

    response = requests.post(url, headers=headers, json=data)

    if response.status_code == 201:
        print(f"INFO: Release successfully created!")
        print(f"INFO: Release url {response.json().get('html_url')}")
    else:
        raise Exception(f"ERROR: Failed to create release\nResponse code: {response.status_code}\n Error message: {response.json()}")


if __name__ == "__main__":
    try:
        check_milestone_and_issue()
        create_github_release()
    except Exception as ex:
        print(ex)
        exit(1)