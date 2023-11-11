#!/usr/bin/python3

import requests
import json
import argparse

parser = argparse.ArgumentParser(description='')
parser.add_argument('--repo', default="ytsaurus/ytsaurus")
parser.add_argument('--token')
parser.add_argument('--github-issue-id', type=int)
parser.add_argument('--github-issue-title')
parser.add_argument('--github-issue-description')

parser.add_argument('--create-issue', action='store_true', default=False)
parser.add_argument('--close-issue', action='store_true', default=False)
parser.add_argument('--update-tags', action='store_true', default=False)

args = parser.parse_args()

headers = {
    'Authorization': f"OAuth {args.token}",
}

github_issue_id_field = "654e39d8384b2913d50ae8a2--githubIssueId"
github_repo_field = "654e39d8384b2913d50ae8a2--githubRepo"

if args.create_issue:
    data = {
        "summary": f"({args.repo}, {args.github_issue_id}) {args.github_issue_title}",
        "queue": "YTSAURUSSUP",
        "description": f"**Repository:** {args.repo}\n\n**Link:** https://github.com/{args.repo}/issues/{args.github_issue_id}\n\n**Description:**\n\n{args.github_issue_description}",
        github_issue_id_field: args.github_issue_id,
        github_repo_field: args.repo,
    }

    response = requests.post('https://st-api.yandex-team.ru/v2/issues/', headers=headers, data=json.dumps(data))

    if not response.ok:
        print("Failed to create issue", response.json())
        exit(1)

    print("Issue was created: ", args.github_issue_id, response.json()["key"])

if args.close_issue:
    # Find issue.
    data = {
        "query": f'YTSAURUSSUP."Github issue id": {args.github_issue_id} AND YTSAURUSSUP."Github repo": "{args.repo}"',
    }

    response = requests.post('https://st-api.yandex-team.ru/v2/issues/_search', headers=headers, data=json.dumps(data))

    if not response.ok:
        print("Failed to search issue", response.json())
        exit(1)

    issues = response.json()

    if len(issues) < 1:
        print("No such issue", args.github_issue_id, args.repo)
        exit(1)

    issue = issues[0]

    print("Issue was found:", issue["key"])

    data = {
        "resolution": "fixed",
        "comment": "Github issue was closed"
    }

    response = requests.post(f"https://st-api.yandex-team.ru/v2/issues/{issue['key']}/transitions/close/_execute", data=json.dumps(data), headers=headers)

    if not response.ok:
        print("Failed to close issue", response.json())
        exit(1)

    print("Issue was closed")
