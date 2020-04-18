bb-wait() {
    local __CONDITION="$1"
    local __TIMEOUT="$2"
    local __COUNTER=$(( $__TIMEOUT ))

    while ! eval "$__CONDITION"
    do
        sleep 1
        if [[ -n "$__TIMEOUT" ]]
        then
            __COUNTER=$(( $__COUNTER - 1 ))
            if (( $__COUNTER <= 0 ))
            then
                bb-log-error "Timeout has been reached during wait for '$__CONDITION'"
                return 1
            fi
        fi
    done
}
