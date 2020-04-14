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
