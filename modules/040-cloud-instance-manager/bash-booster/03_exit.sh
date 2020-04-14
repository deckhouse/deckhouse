BB_EXIT_CODE=0

bb-exit() {
    local CODE=$(( $1 ))
    local MSG="$2"
    bb-exit-helper $CODE "$MSG"
}

bb-exit-on-error() {
    if bb-error?
    then
        local MSG="$1"
        bb-exit-helper $BB_ERROR "$MSG"
    fi
}

bb-exit-helper() {
    local CODE=$(( $1 ))
    local MSG="$2"
    if (( $CODE == 0 ))
    then
        bb-log-info "$MSG"
    else
        bb-log-error "$MSG"
        bb-log-callstack 3
    fi
    BB_EXIT_CODE=$CODE
    exit $CODE
}
