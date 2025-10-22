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

