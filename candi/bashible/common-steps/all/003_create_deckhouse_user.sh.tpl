# Copyright 2021 Flant JSC
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


create_user_and_group() {

    local username="$1"
    local userid="$2"
    local groupname="$3"
    local groupid="$4"

    uid="$(id -u "${username}" 2>/dev/null)"
    gid="$(getent group "${groupname}" | cut -d: -f3)"

    if [ "$uid" == "$userid" ] && [ "$gid" == "$groupid" ]; then
        return
    fi

    if [ -n "$uid" ] || [ -n "$gid" ]; then
        bb-log-warning "user or group already exists with different id: uid=${uid}, gid=${gid}"
        return
    fi

    groupadd -g "$groupid" "$groupname"
    useradd -u "$userid" -g "$groupname" "$username"
}

create_user_and_group deckhouse 64535 deckhouse 64535
