# Bash Booster 0.6 <http://www.bashbooster.net>
# =============================================
#
# Copyright (c) 2014, Dmitry Vakhrushev <self@kr41.net> and Contributors
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#


# This variable can be used to provide additional commands to run by Augeas.
bb-var BB_AUGEAS_EXTRA_COMMANDS ""

bb-augeas?() {
    bb-exe? augtool
}

bb-augeas-get-path() {
    local ABSOLUTE_FILE_PATH="$1"
    local SETTING="$2"

    echo "/files$ABSOLUTE_FILE_PATH/$SETTING"
}

bb-augeas-file-supported?() {
    local ABSOLUTE_FILE_PATH="$1"
    local OUTPUT=

    # Define the helper function
    bb-ext-augeas 'bb-augeas-file-supported?-helper' <<EOF
$BB_AUGEAS_EXTRA_COMMANDS
match /augeas/load/*/incl '$ABSOLUTE_FILE_PATH'
print '/augeas/files$ABSOLUTE_FILE_PATH[count(error) = 0]/*'
EOF

    # Run the helper function
    OUTPUT="$(bb-augeas-file-supported?-helper)"
    bb-error? && bb-assert false "Failed to execute augeas"

    # File is supported if output is not empty.
    [[ -n "$OUTPUT" ]]
}

bb-augeas-get() {
    local ABSOLUTE_FILE_PATH="$1"
    local SETTING="$2"
    local AUG_PATH="$(bb-augeas-get-path "$ABSOLUTE_FILE_PATH" "$SETTING")"
    local VALUE=

    # Validate the specified file
    bb-augeas-file-supported? "$ABSOLUTE_FILE_PATH" || { bb-log-error "Cannot get value from unsupported file '$ABSOLUTE_FILE_PATH'"; return 1; }

    # Define the helper function
    bb-ext-augeas 'bb-augeas-get-helper' <<EOF
$BB_AUGEAS_EXTRA_COMMANDS
get '$AUG_PATH'
EOF

    # Run the helper function
    VALUE="$(bb-augeas-get-helper)"
    if bb-error?
    then
        bb-log-error "An error occured while getting value of '$SETTING' from $ABSOLUTE_FILE_PATH"
        return $BB_ERROR
    fi

    # Handle the result
    if [[ $VALUE == *" = "* ]]
    then
         # Value of the setting has been found
         # Output is in the form /files/.../<Setting> = Value
         VALUE="${VALUE#*=}"
         VALUE="${VALUE// }" # Remove leading spaces
         VALUE="${VALUE%%}"  # Remove trailing spaces
    elif [[ $VALUE == *" (none)"* ]]
    then
        # Setting has empty value
        VALUE=""
    else
        # Setting not found/set
        VALUE=""
    fi

    echo "$VALUE"
}

bb-augeas-match?() {
    local ABSOLUTE_FILE_PATH="$1"
    local SETTING="$2"
    local VALUE="$3"
    local AUG_PATH="$(bb-augeas-get-path "$ABSOLUTE_FILE_PATH" "$SETTING")"
    local OUTPUT

    # Validate the specified file
    bb-augeas-file-supported? "$ABSOLUTE_FILE_PATH" || { bb-log-error "Cannot match value from unsupported file '$ABSOLUTE_FILE_PATH'"; return 1; }

    # Define the helper function
    bb-ext-augeas 'bb-augeas-match-helper' <<EOF
$BB_AUGEAS_EXTRA_COMMANDS
match '$AUG_PATH' "$VALUE"
EOF

    # Run the helper function
    OUTPUT="$(bb-augeas-match-helper)"
    if bb-error?
    then
        bb-log-error "An error occured while verifying if '$SETTING' matches '$VALUE' ($AUG_PATH)"
        return $BB_ERROR
    fi

    # Check output
    # When there is a match, the output is in the form:
    #     /files/<File path>
    [[ "$OUTPUT" == "/files/"* ]]
}

bb-augeas-set() {
    local ABSOLUTE_FILE_PATH="$1"
    local SETTING="$2"
    local VALUE="$3"
    local AUG_PATH="$(bb-augeas-get-path "$ABSOLUTE_FILE_PATH" "$SETTING")"
    local OUTPUT=
    shift 3

    # Validate the specified file
    bb-augeas-file-supported? "$ABSOLUTE_FILE_PATH" || { bb-log-error "Cannot set value to unsupported file '$ABSOLUTE_FILE_PATH'"; return 1; }

    # Define the helper function
    bb-ext-augeas 'bb-augeas-set-helper' <<EOF
$BB_AUGEAS_EXTRA_COMMANDS
set "$AUG_PATH" "$VALUE"
save
EOF

    # Run the helper function
    OUTPUT="$(bb-augeas-set-helper)"
    if bb-error?
    then
        bb-log-error "An error occured while setting value of '$SETTING' to '$VALUE' ($ABSOLUTE_FILE_PATH)"
        return $BB_ERROR
    fi

    # Raise events if file changed
    if [[ "$OUTPUT" == "Saved "* ]]
    then
        bb-event-delay "$@"
        bb-event-fire "bb-augeas-file-changed" "$ABSOLUTE_FILE_PATH"
    fi
}

