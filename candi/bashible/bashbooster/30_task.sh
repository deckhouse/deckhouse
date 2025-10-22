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

declare -A BB_TASK_FUNCS
declare -a BB_TASK_CONTEXT

bb-task-def() {
    local NAME="$1"
    local FUNC="${2-${NAME}}"
    BB_TASK_FUNCS[$NAME]="$FUNC"
}

bb-task-run() {
    BB_TASK_CONTEXT[${#BB_TASK_CONTEXT[@]}]="$( bb-tmp-file )"
    bb-task-depends "$@"
    unset BB_TASK_CONTEXT[${#BB_TASK_CONTEXT[@]}-1]
}

bb-task-depends() {
    local CONTEXT="${BB_TASK_CONTEXT[${#BB_TASK_CONTEXT[@]}-1]}"
    local CODE
    local NAME
    local TASK

    if [[ ! -f "$CONTEXT" ]]
    then
        bb-exit $BB_ERROR_TASK_BAD_CONTEXT "Cannot run tasks. Bad context"
    fi
    for NAME in "$@"
    do
        if [[ -z $( cat "$CONTEXT" | grep "^$NAME$" ) ]]
        then
            bb-log-info "Running task '$NAME'..."
            TASK=${BB_TASK_FUNCS[$NAME]}
            if [[ -z "$TASK" ]]
            then
                bb-exit $BB_ERROR_TASK_UNDEFINED "Undefined task '$NAME'"
            fi
            $TASK
            CODE=$?
            if (( $CODE != 0 ))
            then
                bb-exit $CODE "Task '$NAME' failed"
            fi
            bb-log-info "Task '$NAME' OK"
        fi
        echo "$NAME" >> "$CONTEXT"
    done
}
