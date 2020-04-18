bb-assert() {
    # Local vars are prefixed to avoid conflicts with ASSERTION expression
    local __ASSERTION="$1"
    local __MESSAGE="${2-Assertion error '$__ASSERTION'}"

    if ! eval "$__ASSERTION"
    then
        bb-exit $BB_ERROR_ASSERT_FAILED "$__MESSAGE"
    fi
}

bb-assert-root() {
    local __MESSAGE="${1-This script must be run as root!}"
    bb-assert '[[ $EUID -eq 0 ]]' "$__MESSAGE"
}

bb-assert-file() {
    local __FILE="$1"
    local __MESSAGE="${2-File '$__FILE' not found}"
    bb-assert '[[ -f $__FILE ]]' "$__MESSAGE"
}

bb-assert-file-readable() {
    local __FILE="$1"
    local __MESSAGE="${2-File '$__FILE' is not readable}"
    bb-assert '[[ -f $__FILE ]] && [[ -r $__FILE ]]' "$__MESSAGE"
}

bb-assert-file-writeable() {
    local __FILE="$1"
    local __MESSAGE="${2-File '$__FILE' is not writeable}"
    bb-assert '[[ -f $__FILE ]] && [[ -w $__FILE ]]' "$__MESSAGE"
}

bb-assert-file-executable() {
    local __FILE="$1"
    local __MESSAGE="${2-File '$__FILE' is not executable}"
    bb-assert '[[ -f $__FILE ]] && [[ -x $__FILE ]]' "$__MESSAGE"
}

bb-assert-dir() {
    local __DIR="$1"
    local __MESSAGE="${2-Directory '$__DIR' not found}"
    bb-assert '[[ -d $__DIR ]]' "$__MESSAGE"
}

bb-assert-var() {
    local __VAR="$1"
    local __MESSAGE="${2-Variable '$__VAR' not set}"
    bb-assert '[[ -n ${!__VAR} ]]' "$__MESSAGE"
}

