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

bb-tmp-init() {
    BB_TMP_DIR="$BB_WORKSPACE/tmp_$( bb-tmp-name )"
    mkdir "$BB_TMP_DIR"
}

bb-tmp-file() {
    local FILENAME="$BB_TMP_DIR/$( bb-tmp-name )"
    touch "$FILENAME"
    echo "$FILENAME"
}

bb-tmp-dir() {
    local DIRNAME="$BB_TMP_DIR/$( bb-tmp-name )"
    mkdir -p "$DIRNAME"
    echo "$DIRNAME"
}

bb-tmp-name() {
    echo "$( date +%s )$RANDOM"
}

bb-tmp-cleanup() {
    rm -rf "$BB_TMP_DIR"
}

