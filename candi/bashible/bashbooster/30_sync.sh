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

bb-var BB_SYNC_UNHANDLED_FILES_STORE "/var/lib/bashible/bashbooster_unhandled_synced_files"

bb-sync-file() {
    local DST_FILE="$1"
    local SRC_FILE="$2"
    shift 2

    local DST_FILE_CHANGED=false

    if [[ "$SRC_FILE" == "-" ]]
    then
        SRC_FILE="$(bb-tmp-file)"
        cat > "$SRC_FILE"
    fi

    if test -f "$BB_SYNC_UNHANDLED_FILES_STORE" && grep -Eq "^${DST_FILE}$" "$BB_SYNC_UNHANDLED_FILES_STORE"; then
        DST_FILE_CHANGED=true
    fi

    if [[ ! -f "$DST_FILE" ]]
    then
        touch "$DST_FILE"
        DST_FILE_CHANGED=true
        echo "$DST_FILE" >> "$BB_SYNC_UNHANDLED_FILES_STORE"
    fi
    if [[ -n "$( diff -q "$SRC_FILE" "$DST_FILE" )" ]]
    then
        cp -f -p "$SRC_FILE" "$DST_FILE"
        DST_FILE_CHANGED=true
        echo "$DST_FILE" >> "$BB_SYNC_UNHANDLED_FILES_STORE"
    fi

    if $DST_FILE_CHANGED
    then
        bb-event-fire "bb-sync-file-changed" "$DST_FILE"
        bb-event-delay "$@"
    fi
}

bb-sync-dir() {
    local TWO_WAY=true
    local -a ARGS=()
    local -i TEST=3    # Test first three arguments

    while (( $# && TEST-- ))
    do
        case "$1" in
            -o|--one-way) TWO_WAY=false ;;
            -t|--two-way) TWO_WAY=true  ;;
            *) ARGS[${#ARGS[@]}]="$1"   ;;
        esac
        shift
    done
    ARGS=( "${ARGS[@]}" "$@" )

    bb-sync-dir-helper "$TWO_WAY" "${ARGS[@]}"
}

bb-sync-dir-helper() {
    local TWO_WAY="$1"
    local DST_DIR="$( cd "$( dirname "$2" )" ; pwd )/$( basename "$2" )"
    local SRC_DIR="$( cd "$3" ; pwd )"
    shift 3

    if [[ ! -d "$DST_DIR" ]]
    then
        mkdir -p "$DST_DIR"
        bb-event-delay "$@"
        bb-event-fire "bb-sync-dir-created" "$DST_DIR"
    fi

    local ORIGINAL_DIR="$( pwd )"
    local NAME

    cd "$SRC_DIR"
    while read -r NAME
    do
        if [[ -f "$SRC_DIR/$NAME" ]]
        then
            bb-sync-file "$DST_DIR/$NAME" "$SRC_DIR/$NAME" "$@"
        elif [[ -d "$SRC_DIR/$NAME" ]]
        then
            bb-sync-dir-helper "$TWO_WAY" "$DST_DIR/$NAME" "$SRC_DIR/$NAME" "$@"
        fi
    done < <( ls -A )

    if $TWO_WAY
    then
        cd "$DST_DIR"
        while read -r NAME
        do
            if [[ ! -e "$SRC_DIR/$NAME" ]]
            then
                local EVENT="bb-sync-file-removed"
                if [[ -d "$DST_DIR/$NAME" ]]
                then
                    EVENT="bb-sync-dir-removed"
                fi
                rm -rf "$DST_DIR/$NAME"
                bb-event-delay "$@"
                bb-event-fire "$EVENT" "$DST_DIR/$NAME"
            fi
        done < <( find . )
    fi

    cd "$ORIGINAL_DIR"
}
