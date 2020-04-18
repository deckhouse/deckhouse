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
