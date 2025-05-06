
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
import re
import requests
from datetime import datetime

# Env vars
defectdojo_host = os.getenv('DEFECTDOJO_HOST')
defectdojo_token = os.getenv('DEFECTDOJO_API_TOKEN')
days_to_keep_dev = int(os.getenv('DEFECTDOJO_DEV_TESTS_ROTATION_DAYS', 3))
days_to_keep_release = int(os.getenv('DEFECTDOJO_UNUPDATED_RELEASE_TESTS_ROTATION_DAYS', 14))
release_versions_to_keep = int(os.getenv('DEFECTDOJO_RELEASE_TESTS_ROTATION_VERSIONS_AMOUNT', 3))

# Static vars
defectdojo_proto = "https://"
defectdojo_api_url = defectdojo_proto+defectdojo_host+"/api/v2/"
defectdojo_deckhouse_images_engagement = "CVE Test: Deckhouse Images"
headers = {"accept": "application/json", "Content-Type": "application/json", "Authorization": "Token "+defectdojo_token}
current_date=datetime.now().date()



def delete_test(test, removed_tests_counter):
    deleted_result = requests.delete(defectdojo_api_url+"tests/"+str(test["id"])+"/", headers=headers)
    if deleted_result.status_code == 204:
        removed_tests_counter += 1
        print("Test: "+str(test["id"])+" "+str(test["title"])+" was successfully removed")
    else:
        print("Test: "+str(test["id"])+" "+str(test["title"])+" was NOT REMOVED, response code: "+str(deleted_result.status_code))
    return removed_tests_counter


def get_releases_to_keep(eng_tests):
    releases_to_keep=[]
    for item in eng_tests:
        if re.match(r"^\d*\.\d*$", item["version"]) and (current_date - datetime.fromisoformat(item["updated"]).date()).days <= days_to_keep_release:
            releases_to_keep.append(item["version"])
    if releases_to_keep:
        # uniquify and sort if list not empty
        releases_to_keep = list(set(releases_to_keep))
        releases_to_keep.sort(reverse=True)
    return releases_to_keep


def get_old_tests():
    removed_tests_counter = 0
    obsolete_tests_counter = 0
    for product in requests.get(defectdojo_api_url+"products", headers=headers).json()["results"]:
        for eng in requests.get(defectdojo_api_url+"engagements", headers=headers, params={"product": product["id"]}).json()["results"]:
            print("======================================================")
            print(f'Product: \"{product["name"]}\", Engagement: \"{eng["name"]}\"')
            eng_tests=requests.get(defectdojo_api_url+"tests", headers=headers, params={"engagement": eng["id"], "limit": "10000", "not_tags": "branch:main,branch:master"}).json()["results"]
            releases_to_keep = get_releases_to_keep(eng_tests)
            print(f'The following release versions for product \"{product["name"]}\" will be kept:')
            print(f'{releases_to_keep}')
            for test in eng_tests:
                #if version == mr* or pr* and older then days_to_keep_dev - delete
                if re.match(r"^mr*|^pr*", test["version"]):
                    if (current_date - datetime.fromisoformat(test["created"]).date()).days > days_to_keep_dev:
                        obsolete_tests_counter += 1
                        removed_tests_counter = delete_test(test, removed_tests_counter)

                #if it is not release version that we should keep - delete
                elif test["version"] not in releases_to_keep:
                    obsolete_tests_counter += 1
                    removed_tests_counter = delete_test(test, removed_tests_counter)

                #if other version - delete as most likely it is from dev branch
                else:
                    obsolete_tests_counter += 1
                    removed_tests_counter = delete_test(test, removed_tests_counter)
    if obsolete_tests_counter > 0:
        print(f'"Obsolete tests were removed: {removed_tests_counter}/{obsolete_tests_counter}')
    else:
        print("Nothing to remove")



if __name__ == "__main__":
    get_old_tests()
