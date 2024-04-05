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

bb-flag-init() {
    BB_FLAG_DIR="$BB_WORKSPACE/flag"
}

bb-flag?() {
    local FLAG="$1"
    [[ -f "$BB_FLAG_DIR/$FLAG" ]]
}

bb-flag-set() {
    local FLAG="$1"
    if [[ ! -d "$BB_FLAG_DIR" ]]
    then
        bb-log-debug "Creating flag directory at '$BB_FLAG_DIR'"
        mkdir "$BB_FLAG_DIR"
    fi
    bb-log-info "Creating flag '$FLAG' at '$BB_FLAG_DIR'"
    touch "$BB_FLAG_DIR/$FLAG"
}

bb-flag-unset() {
    local FLAG="$1"
    [[ ! -f "$BB_FLAG_DIR/$FLAG" ]] || rm "$BB_FLAG_DIR/$FLAG"
}

bb-flag-clean() {
    bb-log-debug "Removing flag directory"
    rm -rf "$BB_FLAG_DIR"
}

bb-flag-cleanup() {
    if [[ -d "$BB_FLAG_DIR" && -z "$( ls "$BB_FLAG_DIR" )" ]]
    then
        bb-log-debug "Flag directory is empty"
        bb-flag-clean
    fi
}
