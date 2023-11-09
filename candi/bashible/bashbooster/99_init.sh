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

bb-workspace-init
bb-tmp-init
bb-event-init
bb-download-init
bb-flag-init


bb-cleanup-update-exit-code() {
    if bb-error? && (( $BB_EXIT_CODE == 0 ))
    then
        BB_EXIT_CODE=$BB_ERROR
    fi
}

bb-cleanup() {
    bb-cleanup-update-exit-code

    bb-event-fire bb-cleanup        ; bb-cleanup-update-exit-code

    bb-flag-cleanup                 ; bb-cleanup-update-exit-code
    bb-event-cleanup                ; bb-cleanup-update-exit-code
    bb-tmp-cleanup                  ; bb-cleanup-update-exit-code
    bb-workspace-cleanup            ; bb-cleanup-update-exit-code

    exit $BB_EXIT_CODE
}

trap bb-cleanup EXIT
