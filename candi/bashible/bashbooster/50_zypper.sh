# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

bb-var BB_ZYPPER_UNHANDLED_PACKAGES_STORE "/var/lib/bashible/bashbooster_unhandled_packages"
bb-var BB_ZYPPER_INSTALL_EXTRA_ARGS ""

bb-zypper?() {
    bb-exe? zypper
}

bb-zypper-package?() {
    local PACKAGE=$1
    rpm -qa "$PACKAGE" &> /dev/null
}

bb-zypper-update() {
    bb-flag? zypper-updated && return 0
    bb-log-info 'Updating zypper cache'
    zypper clean
    zypper refresh
    bb-flag-set zypper-updated
}

bb-zypper-install() {
    PACKAGES_TO_INSTALL=()
    for PACKAGE in "$@"
    do
        local NEED_FIRE=false
        if test -f "$BB_ZYPPER_UNHANDLED_PACKAGES_STORE" && grep -Eq "^${PACKAGE}$" "$BB_ZYPPER_UNHANDLED_PACKAGES_STORE"; then
            NEED_FIRE=true
        fi
        if ! bb-zypper-package? "$PACKAGE"
        then
            PACKAGES_TO_INSTALL+=("$PACKAGE")
        fi
    done

    if [ "${#PACKAGES_TO_INSTALL[@]}" -gt "0" ]
    then
        bb-zypper-update
        bb-log-info "Installing packages '${PACKAGES_TO_INSTALL[@]}'"

        zypper install $BB_ZYPPER_INSTALL_EXTRA_ARGS -y ${PACKAGES_TO_INSTALL[@]}
        bb-exit-on-error "Failed to install packages '${PACKAGES_TO_INSTALL[@]}'"
        printf '%s\n' "${PACKAGES_TO_INSTALL[@]}" >> "$BB_ZYPPER_UNHANDLED_PACKAGES_STORE"
        NEED_FIRE=true
    fi
    if [[ "$NEED_FIRE" == "true" ]]; then
        bb-event-fire "bb-package-installed" "$PACKAGE"
    fi
}

bb-zypper-remove() {
    for PACKAGE in "$@"; do
        if bb-zypper-package? "$PACKAGE"
        then
            bb-zypper-update
            bb-log-info "Removing package '$PACKAGE'"
            zypper remove -y "$PACKAGE"
            bb-exit-on-error "Failed to remove package '$PACKAGE'"
        fi
    done
}
