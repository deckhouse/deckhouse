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

bb-var BB_APT_UNHANDLED_PACKAGES_STORE "/var/lib/bashible/bashbooster_unhandled_packages"

bb-apt?() {
    bb-exe? apt-get
}

bb-apt-repo?() {
    local REPO_PART=$1
    cat /etc/apt/sources.list /etc/apt/sources.list.d/* 2> /dev/null | grep -v '^#' | grep -qw "$REPO_PART"
}

bb-apt-key-add() {
    apt-key add -
}

bb-apt-repo-add() {
    local REPO_HASH="$(sed -E -e 's/[ \t]+/;;/g' <<< "${@}")"
    local REPO_DOMAIN="$(sed -E -e 's/.*http(s)?:\/\/([^/ \t]+)\/.*/\2/' <<< $2)"

    if ! cat /etc/apt/sources.list /etc/apt/sources.list.d/* 2> /dev/null | sed -E -e 's/#.*//g' -e 's/[ \t]+/;;/g' | grep -q $REPO_HASH
    then
        echo "${@}" >> "/etc/apt/sources.list.d/${REPO_DOMAIN}.list"
        bb-flag-unset apt-updated
    fi
}

bb-apt-package?() {
    IFS='=' read -ra PACKAGE_ARRAY <<< "$1"
    local PACKAGE="${PACKAGE_ARRAY[0]}"
    local VERSION_DESIRED="${PACKAGE_ARRAY[1]-}"

    if [ -z "$VERSION_DESIRED" ]; then
        dpkg -s "$PACKAGE" 2> /dev/null | grep -q '^Status:.\+installed'
    else
        VERSION_INSTALLED="$(dpkg -l "$PACKAGE" 2>/dev/null | grep -E "(hi|ii)\s+($PACKAGE)" | awk '{print $3}')" || return 1

        VERSION_REGEX="$(sed -E -e 's/\*/[a-zA-Z0-9_+~:-]*/' -e 's/(.*)/^\1$/' <<< $VERSION_DESIRED)"
        grep -q "$VERSION_REGEX" <<< $VERSION_INSTALLED
    fi
}

bb-apt-update() {
    export DEBIAN_FRONTEND=noninteractive
    bb-flag? apt-updated && return 0
    bb-log-info 'Updating apt cache'
    apt-get update
    bb-flag-set apt-updated
}

bb-apt-dist-upgrade() {
    export DEBIAN_FRONTEND=noninteractive
    bb-apt-update
    bb-log-info 'Processing dist-upgrade'
    apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" dist-upgrade -y
}

bb-apt-install() {
    PACKAGES_TO_INSTALL=()

    local FORCE=false

    export DEBIAN_FRONTEND=noninteractive

    if [[ "$1" == "--force" ]]; then
      FORCE=true
      shift
    fi

    for PACKAGE in "$@"
    do
        local NEED_FIRE=false
        if test -f "$BB_APT_UNHANDLED_PACKAGES_STORE" && grep -Eq "^${PACKAGE}$" "$BB_APT_UNHANDLED_PACKAGES_STORE"; then
            NEED_FIRE=true
        fi

        if [[ "$FORCE" == "true" ]] || ! bb-apt-package? "$PACKAGE"; then
            PACKAGES_TO_INSTALL+=("$PACKAGE")
        fi
    done

    if [ "${#PACKAGES_TO_INSTALL[@]}" -gt "0" ]
    then
        bb-apt-update
        bb-log-info "Installing packages '${PACKAGES_TO_INSTALL[@]}'"
        apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install --allow-change-held-packages --allow-downgrades -y ${PACKAGES_TO_INSTALL[@]}
        bb-exit-on-error "Failed to install packages '${PACKAGES_TO_INSTALL[@]}'"
        printf '%s\n' "${PACKAGES_TO_INSTALL[@]}" >> "$BB_APT_UNHANDLED_PACKAGES_STORE"
        NEED_FIRE=true
    fi
    if [[ "$NEED_FIRE" == "true" ]]; then
        bb-event-fire "bb-package-installed" "$PACKAGE"
    fi
}

bb-apt-remove() {
    export DEBIAN_FRONTEND=noninteractive
    local PACKAGES_TO_REMOVE=( )

    for PACKAGE in "$@"
    do
        if bb-apt-package? "$PACKAGE"
        then
            PACKAGES_TO_REMOVE+=( "$PACKAGE" )
        fi
    done

    if [ "${#PACKAGES_TO_REMOVE[@]}" -gt 0 ]; then
        bb-log-info "Removing packages '${PACKAGES_TO_REMOVE[@]}'"
        apt-get remove -y --allow-change-held-packages ${PACKAGES_TO_REMOVE[@]}
        bb-exit-on-error "Failed to remove packages '${PACKAGES_TO_REMOVE[@]}'"
        for i in ${PACKAGES_TO_REMOVE[@]}; do
            bb-event-fire "bb-package-removed" "$i"
        done
    fi
}

bb-apt-autoremove() {
    export DEBIAN_FRONTEND=noninteractive
    bb-log-info 'Autoremoving unused packages'
    apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" --purge -y autoremove
}

bb-apt-package-upgrade?() {
    export DEBIAN_FRONTEND=noninteractive
    bb-apt-update

    local PACKAGE=$1
    local OUTPUT="$(
        apt-cache policy "$PACKAGE" | awk -c '
            /Installed: / { installed = $2 }
            /Candidate: / {
                if (installed != "(none)" && installed != $2) {
                    print installed " " $2
                }
            }
        '
    )"

    # Note: No upgrade available is reported for a non-installed package
    [[ -n "$OUTPUT" ]]
}

bb-apt-upgrade() {
    export DEBIAN_FRONTEND=noninteractive
    for PACKAGE in "$@"
    do
        if bb-apt-package-upgrade? "$PACKAGE"
        then
            bb-log-info "Upgrading package '$PACKAGE'"
            bb-event-fire "bb-package-pre-upgrade" "$PACKAGE"
            apt-get upgrade -y "$PACKAGE"
            bb-exit-on-error "Failed to upgrade package '$PACKAGE'"
            bb-event-fire "bb-package-post-upgrade" "$PACKAGE"
        fi
    done
}
