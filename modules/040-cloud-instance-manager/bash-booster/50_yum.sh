bb-var BB_YUM_UPDATED false

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
    $BB_YUM_UPDATED && return 0
    bb-log-info 'Updating yum cache'
    yum clean all
    yum makecache
    BB_YUM_UPDATED=true
}

bb-yum-install() {
    for PACKAGE in "$@"
    do
        if ! bb-yum-package? "$PACKAGE"
        then
            bb-yum-update
            bb-log-info "Installing package '$PACKAGE'"
            yum install -y "$PACKAGE"
            bb-exit-on-error "Failed to install package '$PACKAGE'"
            bb-event-fire "bb-package-installed" "$PACKAGE"
        fi
    done
}
