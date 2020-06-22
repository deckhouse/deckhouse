bb-var BB_YUM_UPDATED false
bb-var BB_YUM_UNHANDLED_PACKAGES_STORE "/var/lib/bashible/bashbooster_unhandled_packages"
bb-var BB_YUM_INSTALL_EXTRA_ARGS ""

bb-yum?() {
    bb-exe? yum
}

bb-yum-repo?() {
    local REPO=$1
    yum -C repolist | grep -Ewq "^(\W)*${REPO}"
}

bb-yum-package?() {
    local PACKAGE=$1
    yum -C list installed "$PACKAGE" &> /dev/null
}

bb-yum-update() {
    bb-flag? yum-updated && return 0
    bb-log-info 'Updating yum cache'
    yum clean all
    yum makecache
    bb-flag-set yum-updated
}

bb-yum-install() {
    PACKAGES_TO_INSTALL=()
    for PACKAGE in "$@"
    do
        local NEED_FIRE=false
        if test -f "$BB_YUM_UNHANDLED_PACKAGES_STORE" && grep -Eq "^${PACKAGE}$" "$BB_YUM_UNHANDLED_PACKAGES_STORE"; then
            NEED_FIRE=true
        fi
        if ! bb-yum-package? "$PACKAGE"
        then
            PACKAGES_TO_INSTALL+=("$PACKAGE")
        fi
    done

    if [ "${#PACKAGES_TO_INSTALL[@]}" -gt "0" ]
    then
        bb-yum-update
        bb-log-info "Installing packages '${PACKAGES_TO_INSTALL[@]}'"
        yum install $BB_YUM_INSTALL_EXTRA_ARGS -y ${PACKAGES_TO_INSTALL[@]}
        bb-yum-versionlock ${PACKAGES_TO_INSTALL[@]}
        bb-exit-on-error "Failed to install packages '${PACKAGES_TO_INSTALL[@]}'"
        printf '%s\n' "${PACKAGES_TO_INSTALL[@]}" >> "$BB_YUM_UNHANDLED_PACKAGES_STORE"
        NEED_FIRE=true
    fi
    if [[ "$NEED_FIRE" == "true" ]]; then
        bb-event-fire "bb-package-installed" "$PACKAGE"
    fi
}

bb-yum-remove() {
    for PACKAGE in "$@"
    do
        if bb-yum-package? "$PACKAGE"
        then
            bb-yum-update
            bb-log-info "Removing package '$PACKAGE'"
            yum remove -y "$PACKAGE"
            bb-exit-on-error "Failed to remove package '$PACKAGE'"
        fi
    done
}

bb-yum-versionlock() {
    for PACKAGE in "$@"
    do
        if ! bb-yum-package? "$PACKAGE"
        then
            bb-log-info "Locking package version of '$PACKAGE'"
            yum versionlock "$PACKAGE"
            bb-exit-on-error "Failed to lock package vetsion of '$PACKAGE'"
        fi
    done
}
