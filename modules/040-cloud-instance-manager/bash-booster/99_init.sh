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
