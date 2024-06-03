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

bb-create_user_and_group() {
    local username="$1"
    local userid="$2"
    local groupname="$3"
    local groupid="$4"

    if id "$username" &>/dev/null; then
        if [ "$(id -u "$username")" -eq "$userid" ] && [ "$(id -g "$username")" -eq "$groupid" ]; then
            bb-log-warning "User $username with UID $userid and GID $groupid already exists. No changes needed."
            return
        fi
    fi

    bb-log-info "Creating user $username with UID $userid and GID $groupid"

    # Check if user already exists
    if getent passwd "$username" > /dev/null 2>&1; then
        bb-log-warning "User $username already exists"
    else
        # Check if group already exists
        if getent group "$groupname" > /dev/null 2>&1; then
            bb-log-warning "Group $groupname already exists"
        else
            # Check if group ID is already exists
            if getent group "$groupid" > /dev/null 2>&1; then
                bb-log-warning "Group ID $groupid is already exists"
                groupname=$(getent group "$groupid" | cut -d: -f1)
            else
                groupadd -g "$groupid" "$groupname"
            fi
        fi

        # Check if user ID is already exists
        if getent passwd "$userid" > /dev/null 2>&1; then
            bb-log-warning "User ID $userid is already exists"
        else
            useradd -m -u "$userid" -g "$groupname" "$username"
        fi
    fi
}
