# Copyright 2025 Flant JSC
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

import os
import sys
import re
from github import Github
from github import Auth

release_number = os.getenv('RELEASE_NUMBER')
gh_token = os.getenv('ACCESS_TOKEN')
gh_repo = "Nikolay1224/TestRepo"

def check_input():
    if not re.fullmatch(r'\d\.[\d]*', release_number):
        raise Exception (f"RELEASE_NUMBER have incorrect value: {release_number}")

def create_milestone(milestone_title):
    milestone_exist = False
    auth = Auth.Token(gh_token)
    g = Github(auth=auth)

    repo = g.get_repo(gh_repo)
    open_milestones = repo.get_milestones(state='open', sort='title', direction='desc')
    print(f"INFO: Searching for open milestone with the same title...")
    for milestone in open_milestones:
        if milestone.title == milestone_title:
            print(f"INFO: Milestone {milestone_title} already exist!\n URL: https://github.com/{gh_repo}/milestone/{milestone.number}")
            milestone_exist = True
            break

    if not milestone_exist:
        print(f"INFO: Open milestone {milestone_title} is not found, lets create it!")
        milestone = repo.create_milestone(title=milestone_title)
        print(f"INFO: Milestone {milestone_title} created!\n URL: https://github.com/{gh_repo}/milestone/{milestone.number}")


if __name__ == "__main__":
    try:
        check_input()
        milestone_title = "v"+release_number+".0"
        print("Milestone title to create: "+milestone_title)
        create_milestone(milestone_title)
    except Exception as ex:
        sys.exit("ERROR: "+ str(ex))
