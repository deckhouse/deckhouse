BB_LOG_DEBUG=1
BB_LOG_INFO=2
BB_LOG_WARNING=3
BB_LOG_ERROR=4

declare -A BB_LOG_LEVEL_NAME
BB_LOG_LEVEL_NAME[$BB_LOG_DEBUG]='DEBUG'
BB_LOG_LEVEL_NAME[$BB_LOG_INFO]='INFO'
BB_LOG_LEVEL_NAME[$BB_LOG_WARNING]='WARNING'
BB_LOG_LEVEL_NAME[$BB_LOG_ERROR]='ERROR'

declare -A BB_LOG_LEVEL_CODE
BB_LOG_LEVEL_CODE['DEBUG']=$BB_LOG_DEBUG
BB_LOG_LEVEL_CODE['INFO']=$BB_LOG_INFO
BB_LOG_LEVEL_CODE['WARNING']=$BB_LOG_WARNING
BB_LOG_LEVEL_CODE['ERROR']=$BB_LOG_ERROR

bb-var BB_LOG_LEVEL $BB_LOG_INFO
bb-var BB_LOG_PREFIX "$( basename "$0" )"
bb-var BB_LOG_TIME 'date +"%Y-%m-%d %H:%M:%S"'
bb-var BB_LOG_FORMAT '${PREFIX} [${LEVEL}] ${MESSAGE}'
bb-var BB_LOG_USE_COLOR false

$BB_LOG_USE_COLOR && BB_LOG_FORMAT="\${COLOR}${BB_LOG_FORMAT}\${NOCOLOR}"

bb-var BB_LOG_FORMAT "$BB_LOG_DEFAULT_FORMAT"

declare -A BB_LOG_COLORS
BB_LOG_COLORS[$BB_LOG_DEBUG]="$( tput bold )$( tput setaf 0 )"  # Dark Gray
BB_LOG_COLORS[$BB_LOG_INFO]="$( tput setaf 2 )"                 # Green
BB_LOG_COLORS[$BB_LOG_WARNING]="$( tput setaf 3 )"              # Brown/Orange
BB_LOG_COLORS[$BB_LOG_ERROR]="$( tput setaf 1 )"                # Red
BB_LOG_COLORS['NC']="$( tput sgr0 )"

bb-log-level-code() {
    local CODE=$(( $BB_LOG_LEVEL ))
    if (( $CODE == 0 ))
    then
        CODE=$(( ${BB_LOG_LEVEL_CODE[$BB_LOG_LEVEL]} ))
    fi
    echo $CODE
}

bb-log-level-name() {
    local NAME="$BB_LOG_LEVEL"
    if (( $BB_LOG_LEVEL != 0 ))
    then
        NAME="${BB_LOG_LEVEL_NAME[$BB_LOG_LEVEL]}"
    fi
    echo $NAME
}

bb-log-prefix() {
    local PREFIX="$BB_LOG_PREFIX"
    local i=2
    while echo "${FUNCNAME[$i]}" | grep -q '^bb-log' || \
          [[ "${FUNCNAME[$i]}" == 'bb-exit' ]] || \
          [[ "${FUNCNAME[$i]}" == 'bb-cleanup' ]]
    do
        i=$(( $i + 1 ))
    done
    if echo "${FUNCNAME[$i]}" | grep -q '^bb-'
    then
        PREFIX=$( echo "${FUNCNAME[$i]}" | awk '{ split($0, PARTS, "-"); print PARTS[1]"-"PARTS[2] }' )
    fi
    echo "$PREFIX"
}

bb-log-msg() {
    local LEVEL_CODE=$(( $1 ))
    if (( $LEVEL_CODE >= $( bb-log-level-code ) ))
    then
        local MESSAGE="$2"
        local PREFIX="$( bb-log-prefix )"
        local TIME="$( eval "$BB_LOG_TIME" )"
        local LEVEL="${BB_LOG_LEVEL_NAME[$LEVEL_CODE]}"
        local COLOR="${BB_LOG_COLORS[$LEVEL_CODE]}"
        local NOCOLOR="${BB_LOG_COLORS['NC']}"
        eval "echo -e $BB_LOG_FORMAT" >&2
    fi
}

bb-log-debug() {
    bb-log-msg $BB_LOG_DEBUG "$*"
}

bb-log-info() {
    bb-log-msg $BB_LOG_INFO "$*"
}

bb-log-warning() {
    bb-log-msg $BB_LOG_WARNING "$*"
}

bb-log-error() {
    bb-log-msg $BB_LOG_ERROR "$*"
}

bb-log-deprecated() {
    local ALTERNATIVE="$1"
    local CURRENT="${2-${FUNCNAME[1]}}"
    bb-log-warning "'$CURRENT' is deprecated, use '$ALTERNATIVE' instead"
}

bb-log-callstack() {
    local FRAME=$(( ${1-"1"} ))
    local MSG="Call stack is:"
    for (( i = $FRAME; i < ${#FUNCNAME[@]}; i++ ))
    do
        MSG="$MSG\n\t${BASH_SOURCE[$i]}:${BASH_LINENO[$i-1]}\t${FUNCNAME[$i]}()"
    done
    bb-log-debug "$MSG"
}
