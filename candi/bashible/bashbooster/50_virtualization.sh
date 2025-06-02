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

bb-dvp-nesting-level() {
    local DVP_NESTING_LEVEL="0"
    local DMI_SOURCE="/sys/devices/virtual/dmi/id/product_sku"
    local FIRST_CHAR=""
    if [ -f "$DMI_SOURCE" ]; then
        read -n 1 FIRST_CHAR < "$DMI_SOURCE"
        if [ "$FIRST_CHAR" -eq "$FIRST_CHAR" 2>/dev/null ]; then
            DVP_NESTING_LEVEL="$FIRST_CHAR"
        fi
    fi

    echo "$DVP_NESTING_LEVEL"
}
