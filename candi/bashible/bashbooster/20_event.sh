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

declare -A BB_EVENT_DEPTH
BB_EVENT_MAX_DEPTH=1000

bb-event-init() {
    BB_EVENT_DIR="$( bb-tmp-dir )"
}

bb-event-on() {
    local EVENT="$1"
    local HANDLER="$2"
    local HANDLERS="$BB_EVENT_DIR/$EVENT.handlers"
    touch "$HANDLERS"
    if [[ -z "$( cat "$HANDLERS" | grep "^$HANDLER\$" )" ]]
    then
        bb-log-debug "Subscribed handler '$HANDLER' on event '$EVENT'"
        echo "$HANDLER" >> "$HANDLERS"
    fi
}

bb-event-off() {
    local EVENT="$1"
    local HANDLER="$2"
    local HANDLERS="$BB_EVENT_DIR/$EVENT.handlers"
    if [[ -f "$HANDLERS" ]]
    then
        bb-log-debug "Removed handler '$HANDLER' from event '$EVENT'"
        cat "$HANDLERS" | grep -v "^$HANDLER\$" > "$HANDLERS" || true
    fi
}

bb-event-fire() {
    [[ -n "$@" ]] || return 0

    local EVENT="$1"
    shift

    BB_EVENT_DEPTH["$EVENT"]=$(( ${BB_EVENT_DEPTH["$EVENT"]} + 1 ))
    if (( ${BB_EVENT_DEPTH["$EVENT"]} >= $BB_EVENT_MAX_DEPTH ))
    then
        bb-exit \
            $BB_ERROR_EVENT_MAX_DEPTH_REACHED \
            "Max recursion depth has been reached on processing event '$EVENT'"
    fi
    if [[ -f "$BB_EVENT_DIR/$EVENT.handlers" ]]
    then
        bb-log-info "Run handlers for event '$EVENT'"
        while read -r HANDLER
        do
            eval "$HANDLER $@"
        done < "$BB_EVENT_DIR/$EVENT.handlers"
    fi
    BB_EVENT_DEPTH["$EVENT"]=$(( ${BB_EVENT_DEPTH["$EVENT"]} - 1 ))
}

bb-event-delay() {
    [[ -n "$@" ]] || return 0

    local EVENTS="$BB_EVENT_DIR/events"
    local EVENT=''

    while (( $# ))
    do
        EVENT+="$( printf "%q " "$1" )"
        shift
    done

    touch "$EVENTS"
    if [[ -z "$( cat "$EVENTS" | grep -Fx "$EVENT" )" ]]
    then
        bb-log-debug "Delayed event '$EVENT'"
        printf "%s\n" "$EVENT" >> "$EVENTS"
    fi
}

bb-event-cleanup() {
    BB_EVENT_DEPTH["__delay__"]=$(( ${BB_EVENT_DEPTH["__delay__"]} + 1 ))
    local EVENTS="$BB_EVENT_DIR/events"
    if (( ${BB_EVENT_DEPTH["__delay__"]} >= $BB_EVENT_MAX_DEPTH ))
    then
        bb-error "Max recursion depth has been reached on processing event '__delay__'"
        rm "$EVENTS"
        return $BB_ERROR_EVENT_MAX_DEPTH_REACHED
    fi
    if [[ -f "$EVENTS" ]]
    then
        local EVENT_LIST="$( bb-tmp-file )"
        cp -f "$EVENTS" "$EVENT_LIST"
        rm "$EVENTS"
        while read -r EVENT
        do
            bb-event-fire $EVENT
        done < "$EVENT_LIST"
        # If any event hadler calls "bb-event-delay", the "$EVENTS" file
        # will be created again and we should repeat this processing
        if [[ -f "$EVENTS" ]]
        then
            bb-event-cleanup
            if bb-error?
            then
                return $BB_ERROR
            fi
        fi
    fi
    BB_EVENT_DEPTH["__delay__"]=$(( ${BB_EVENT_DEPTH["__delay__"]} - 1 ))
}
