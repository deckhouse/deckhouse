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

bb-var BB_WORKSPACE ".bb-workspace"

bb-workspace-init() {
    bb-log-debug "Initializing workspace at '$BB_WORKSPACE'"
    if [[ ! -d "$BB_WORKSPACE" ]]
    then
        mkdir -p "$BB_WORKSPACE" || bb-exit \
            $BB_ERROR_WORKSPACE_CREATION_FAILED \
            "Failed to initialize workspace at '$BB_WORKSPACE'"
    fi
    # Ensure BB_WORKSPACE stores absolute path
    BB_WORKSPACE="$( cd "$BB_WORKSPACE" ; pwd )"
}

bb-workspace-cleanup() {
    bb-log-debug "Cleaning up workspace at '$BB_WORKSPACE'"
    if [[ -z "$( ls "$BB_WORKSPACE" )" ]]
    then
        bb-log-debug "Workspace is empty. Removing"
        rm -rf "$BB_WORKSPACE"
    else
        bb-log-debug "Workspace is not empty"
    fi
}
