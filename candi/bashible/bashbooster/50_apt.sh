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
    local PACKAGE="$(cut -d= -f1 <<< $1)"
    local VERSION_DESIRED="$(cut -d= -f2 <<<  $1)"

    if [ -z "$VERSION_DESIRED" ]; then
        dpkg -s "$PACKAGE" 2> /dev/null | grep -q '^Status:.\+installed'
    else
        VERSION_INSTALLED="$(dpkg-query -W $PACKAGE 2> /dev/null | awk '{print $2}')" || return 1

        VERSION_REGEX="$(sed -E -e 's/\*/[a-zA-Z0-9_-+~:]*/' -e 's/(.*)/^\1$/' <<< $VERSION_DESIRED)"
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
    export DEBIAN_FRONTEND=noninteractive
    for PACKAGE in "$@"
    do
        if ! bb-apt-package? "$PACKAGE"
        then
            bb-apt-update
            bb-log-info "Installing package '$PACKAGE'"
            apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -y "$PACKAGE"
            bb-apt-hold $PACKAGE
            bb-exit-on-error "Failed to install package '$PACKAGE'"
            bb-event-fire "bb-package-installed" "$PACKAGE"
        fi
    done
}

bb-apt-remove() {
    export DEBIAN_FRONTEND=noninteractive
    for PACKAGE in "$@"
    do
        if bb-apt-package? "$PACKAGE"
        then
            bb-log-info "Removing package '$PACKAGE'"
            apt-get remove -y "$PACKAGE"
            bb-exit-on-error "Failed to remove package '$PACKAGE'"
            bb-event-fire "bb-package-removed" "$PACKAGE"
        fi
    done
}

bb-apt-autoremove() {
    export DEBIAN_FRONTEND=noninteractive
    bb-log-info 'Autoremoving unused packages'
    apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" --purge -y autoremove
}

bb-apt-hold?() {
    dpkg -s "$1" 2> /dev/null | grep -q '^Status:.\+installed'
}

bb-apt-hold() {
    for PACKAGE in "$@"
    do
        apt-mark hold "$PACKAGE"
    done
}

bb-apt-unhold() {
    for PACKAGE in "$@"
    do
        apt-mark unhold "$PACKAGE"
    done
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

