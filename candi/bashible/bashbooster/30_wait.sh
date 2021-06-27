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
