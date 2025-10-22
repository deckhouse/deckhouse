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

bb-var BB_DOWNLOAD_WGET_OPTIONS '-nv'

bb-download-init() {
    BB_DOWNLOAD_DIR="$BB_WORKSPACE/download"
}

bb-download() {
    if [[ ! -d "$BB_DOWNLOAD_DIR" ]]
    then
        bb-log-debug "Creating download directory at '$BB_DOWNLOAD_DIR'"
        mkdir "$BB_DOWNLOAD_DIR"
    fi

    local URL="$1"
    local TARGET="${2-$( basename "$URL" )}"
    local FORCE="${3-false}"
    TARGET="$BB_DOWNLOAD_DIR/$TARGET"
    echo "$TARGET"
    if [[ -f "$TARGET" ]] && ! $FORCE
    then
        return 0
    fi

    bb-log-info "Downloading $URL"
    wget $BB_DOWNLOAD_WGET_OPTIONS -O "$TARGET" "$URL"
    if bb-error?
    then
        bb-log-error "An error occurs while downloading $URL"
        return $BB_ERROR
    fi
}

bb-download-clean() {
    rm -rf "$BB_DOWNLOAD_DIR"
}
