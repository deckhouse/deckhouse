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

