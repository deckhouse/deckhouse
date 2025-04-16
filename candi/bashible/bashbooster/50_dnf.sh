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

bb-var BB_DNF_UPDATED false
bb-var BB_DNF_UNHANDLED_PACKAGES_STORE "/var/lib/bashible/bashbooster_unhandled_packages"
bb-var BB_DNF_INSTALL_EXTRA_ARGS ""

bb-check-dnf() {
    if bb-exe? dnf; then
        echo "dnf"
    elif bb-exe? yum; then
        echo "yum"
    else
        bb-log-error "found neither dnf nor yum"
        exit 1
    fi
}

bb-dnf?() {
    bb-set-proxy
    trap bb-unset-proxy RETURN
    bb-exe? dnf
}

bb-dnf-repo?() {
    bb-set-proxy
    trap bb-unset-proxy RETURN
    dnf repolist | grep -Ew "^(\W)*${1}"
}

bb-dnf-package?() {
    bb-set-proxy
    trap bb-unset-proxy RETURN
    dnf list installed "$1" &>/dev/null
}

bb-dnf-update() {
    bb-set-proxy
    trap bb-unset-proxy RETURN
    bb-flag? dnf-updated && return 0
    bb-log-info "Updating dnf cache"
    dnf clean all
    dnf makecache
    bb-flag-set dnf-updated
}

bb-dnf-install() {
    if [ "$(bb-check-dnf)" = "yum" ]; then
        bb-log-info "bb-dnf-install downgraded to yum"
        bb-yum-install "$@"
        return
    fi
    local PACKAGES_TO_INSTALL=()
    local NEED_FIRE=false
    for PACKAGE in "$@"; do
        if test -f "$BB_DNF_UNHANDLED_PACKAGES_STORE" && grep -Eq "^${PACKAGE}$" "$BB_DNF_UNHANDLED_PACKAGES_STORE"; then
            NEED_FIRE=true
        fi
        if ! bb-dnf-package? "$PACKAGE"; then
            PACKAGES_TO_INSTALL+=("$PACKAGE")
        fi
    done

    if [ "${#PACKAGES_TO_INSTALL[@]}" -gt 0 ]; then
        bb-dnf-update
        bb-log-info "Installing packages '${PACKAGES_TO_INSTALL[@]}'"
        bb-set-proxy
        trap bb-unset-proxy RETURN
        dnf install $BB_DNF_INSTALL_EXTRA_ARGS -y "${PACKAGES_TO_INSTALL[@]}"
        bb-exit-on-error "Failed to install packages '${PACKAGES_TO_INSTALL[@]}'"
        printf '%s\n' "${PACKAGES_TO_INSTALL[@]}" >> "$BB_DNF_UNHANDLED_PACKAGES_STORE"
        NEED_FIRE=true
    fi

    if [[ "$NEED_FIRE" == "true" ]]; then
        bb-event-fire "bb-package-installed" "$PACKAGE"
    fi
}

bb-dnf-remove() {
    if [ "$(bb-check-dnf)" = "yum" ]; then
        bb-log-info "bb-dnf-remove downgraded to yum"
        bb-yum-remove "$@"
        return
    fi
    bb-set-proxy
    trap bb-unset-proxy RETURN
    for PACKAGE in "$@"; do
        if bb-dnf-package? "$PACKAGE"; then
            bb-dnf-update
            bb-log-info "Removing package '$PACKAGE'"
            dnf remove -y "$PACKAGE"
            bb-exit-on-error "Failed to remove package '$PACKAGE'"
            bb-event-fire "bb-package-removed" "$PACKAGE"
        fi
    done
}
