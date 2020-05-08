bb-template() {
    local TEMPLATE="$1"
    eval "echo \"$( < "$TEMPLATE" )\""
}
