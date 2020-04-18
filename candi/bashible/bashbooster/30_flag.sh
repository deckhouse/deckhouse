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
        bb-log-debug "Creating flag directory at '$BB_DOWNLOAD_DIR'"
        mkdir "$BB_FLAG_DIR"
    fi
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
