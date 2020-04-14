bb-sync-file() {
    local DST_FILE="$1"
    local SRC_FILE="$2"
    shift 2

    local DST_FILE_CHANGED=false

    if [[ ! -f "$DST_FILE" ]]
    then
        touch "$DST_FILE"
        DST_FILE_CHANGED=true
    fi
    if [[ -n "$( diff -q "$SRC_FILE" "$DST_FILE" )" ]]
    then
        cp -f -p "$SRC_FILE" "$DST_FILE"
        DST_FILE_CHANGED=true
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
