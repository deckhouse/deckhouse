
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
import requests
from datetime import datetime

# Env vars
defectdojo_host = os.getenv('DEFECTDOJO_HOST')
defectdojo_token = os.getenv('DEFECTDOJO_API_TOKEN')
days_to_keep = int(os.getenv('DEFECTDOJO_DEV_TESTS_ROTATION_DAYS', 7))

# Static vars
defectdojo_proto = "https://"
defectdojo_api_url = defectdojo_proto+defectdojo_host+"/api/v2/"
defectdojo_deckhouse_images_engagement = "CVE Test: Deckhouse Images"
headers = {"accept": "application/json", "Content-Type": "application/json", "Authorization": "Token "+defectdojo_token}
current_date=datetime.now().date()


def get_old_tests():
    engage_id = requests.get(defectdojo_api_url+"engagements", headers=headers, params={"name": defectdojo_deckhouse_images_engagement}).json()["results"][0]["id"]
    old_dev_tests = requests.get(defectdojo_api_url+"tests", headers=headers, params={"engagement": engage_id, "limit": "10000", "not_tag": "main"}).json()["results"]
    return old_dev_tests

def remove_old_tests(old_dev_tests):
    old_tests_counter = 0
    for test in old_dev_tests:
        if (current_date - datetime.fromisoformat(test["created"]).date()).days > days_to_keep:
            deleted_result = requests.delete(defectdojo_api_url+"tests/"+str(test["id"])+"/", headers=headers)
            if deleted_result.status_code == 204:
                old_tests_counter += 1
                print("Test: "+str(test["id"])+" "+str(test["title"])+" was successfully removed")
            else:
                print("Test: "+str(test["id"])+" "+str(test["title"])+" was NOT REMOVED, response code: "+str(deleted_result.status_code))
    if old_tests_counter > 0:
        print("Dev tests were removed: "+str(old_tests_counter))
    else:
        print("Nothing to remove as there are no dev tests older than "+str(days_to_keep)+" days")


if __name__ == "__main__":
    remove_old_tests(get_old_tests())
