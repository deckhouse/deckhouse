declare -A BB_EXT_BODIES

bb-ext-python() {
    local NAME="$1"
    BB_EXT_BODIES["$NAME"]="$( cat )"

    eval "$NAME() { python -c \"\${BB_EXT_BODIES[$NAME]}\" \"\$@\"; }"
}

# Additional parameters to be passed during invokation of Augeas.
bb-var BB_AUGEAS_PARAMS ""

# Root directory for Augeas.
bb-var BB_AUGEAS_ROOT "/"

bb-ext-augeas() {
    local NAME="$1"
    BB_EXT_BODIES["$NAME"]="$( cat )"

    eval "$NAME() {
    eval \"augtool -r \\\"\$BB_AUGEAS_ROOT\\\" \$BB_AUGEAS_PARAMS\" <<EOF
\${BB_EXT_BODIES[$NAME]}
EOF
}"
}

