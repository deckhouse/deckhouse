bb-var BB_APT_UPDATED false

bb-apt?() {
    bb-exe? apt-get
}

bb-apt-repo?() {
    local REPO=$1
    cat /etc/apt/sources.list /etc/apt/sources.list.d/* 2> /dev/null | grep -v '^#' | grep -qw "$REPO"
}

bb-apt-package?() {
    local PACKAGE="$(cut -d= -f1 <<<  $1)"
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
    $BB_APT_UPDATED && return 0
    bb-log-info 'Updating apt cache'
    apt-get update
    BB_APT_UPDATED=true
}

bb-apt-install() {
    for PACKAGE in "$@"
    do
        if ! bb-apt-package? "$PACKAGE"
        then
            bb-apt-update
            bb-log-info "Installing package '$PACKAGE'"
            bb-apt-unhold $PACKAGE
            apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -y "$PACKAGE"
            bb-exit-on-error "Failed to install package '$PACKAGE'"
            bb-event-fire "bb-package-installed" "$PACKAGE"
        fi
    done
}

bb-apt-remove() {
    for PACKAGE in "$@"
    do
        if bb-apt-package? "$PACKAGE"
        then
            bb-apt-update
            bb-log-info "Removing package '$PACKAGE'"
            apt-get remove -y "$PACKAGE"
            bb-exit-on-error "Failed to remove package '$PACKAGE'"
            bb-event-fire "bb-package-removed" "$PACKAGE"
        fi
    done
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

