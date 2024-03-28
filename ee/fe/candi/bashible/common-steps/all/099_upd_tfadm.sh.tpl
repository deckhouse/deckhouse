# Copyright 2024 Flant JSC
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

TFADM_OLD_PUB='ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDTXjTmx3hq2EPDQHWSJN7By1VNFZ8colI5tEeZDBVYAe9Oxq4FZsKCb1aGIskDaiAHTxrbd2efoJTcPQLBSBM79dcELtqfKj9dtjy4S1W0mydvWb2oWLnvOaZX/H6pqjz8jrJAKXwXj2pWCOzXerwk9oSI4fCE7VbqsfT4bBfv27FN4/Vqa6iWiCc71oJopL9DldtuIYDVUgOZOa+t2J4hPCCSqEJK/r+ToHQbOWxbC5/OAufXDw2W1vkVeaZUur5xwwAxIb3wM3WoS3BbwNlDYg9UB2D8+EZgNz1CCCpSy1ELIn7q8RnrTp0+H8V9LoWHSgh3VCWeW8C/MnTW90IR'
TFADM_PUB='cert-authority,principals="tfadm" ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDIg88Qs3x5KR6IQnKYgif1hCig08hYh4eHrriCp4916KpjocdYgq7/TWTeeiJktEEkSo0E+8VJjW0IRk5qdBzPPGRWeSpktf18u4lrq5gWC1OkdDnIgioncxGihA1+ueclQj+eaJZRSh5E8//AROKBJSfAeN6sbn0SqCbZj3cqjR8bgDe4UgdPyXOpJ01iVWgzB0Bk8FHX3NRHdVwZDt7pBRX5Dwvd8z5kifxfWVOC0M5l9tH4dvdHNgthVYTTrIxC7fpAMNul4foe1+2iMg0GAe25MMmlgquWOK91yT2VSfvngsXpx8vb0juKS3Peq/B3TjAjmLxJBVxXx/rQA+AF7J1m6WgCB1bGNPl6Yc86v3SUdScACX+Qj147E5EzsCZbXRDiTq3GQQ9EnnmkK73obZrHkCeSSbH2rWln1IzzbgCc1R0yXpBT1Scjyx0ld31C5cSNq+sL21C0govCtSa93hgxggBUFdQ2a4c7U9knNB4khOF7nTtK543NDsJhXByjF6D484LTSV6Vrg5dbGFFxolaS++rgThtyUkH8BecuKFtxqwx+qxYlqAqO0E1QaUfEXbYtDQAFbcn6AcgU+M633uxzmvp9IYeAp6cy7T5Bbwu+jsErkviEkOwrASTMkJP4IXT6A3RLMNO0TvQwd9vP+kYL8Nk7a2QEHCCfwvpFw=='

for HOME_DIR in $(getent passwd | cut -f6 -d ':' |sort |uniq); do
  AUTHORIZED_KEYS_PATH="${HOME_DIR}/.ssh/authorized_keys"
  if [ $(grep -lirs "${TFADM_OLD_PUB}" ${AUTHORIZED_KEYS_PATH}) ]; then
    bb-log-info "Old tfadm key was found in '${AUTHORIZED_KEYS_PATH}',  updating..."
    sed -i "s|${TFADM_OLD_PUB}|$TFADM_PUB|g" "${AUTHORIZED_KEYS_PATH}"
  fi
done