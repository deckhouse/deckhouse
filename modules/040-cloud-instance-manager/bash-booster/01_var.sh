bb-var() {
    local VAR_NAME=$1
    local DEFAULT=$2
    if [[ -z "${!VAR_NAME}" ]]
    then
        eval "$VAR_NAME='$DEFAULT'"
    fi
}
