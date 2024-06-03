# Bash Booster 0.6 <http://www.bashbooster.net>
# =============================================
#
# Copyright (c) 2014, Dmitry Vakhrushev <self@kr41.net> and Contributors
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

bb-create_user_and_group() {
    local username="$1"
    local userid="$2"
    local groupname="$3"
    local groupid="$4"

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
