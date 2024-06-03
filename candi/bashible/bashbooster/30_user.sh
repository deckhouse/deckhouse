bb-create_user_and_group() {
    local username="$1"
    local userid="$2"
    local groupname="$3"
    local groupid="$4"

    if id "$username" &>/dev/null; then
        if [ "$(id -u "$username")" -eq "$userid" ] && [ "$(id -g "$username")" -eq "$groupid" ]; then
            bb-log-warning "User $username with UID $userid and GID $groupid already exists. No changes needed."
            return
        fi
    fi

    bb-log-info "Creating user $username with UID $userid and GID $groupid"

    # Check if user already exists
    if getent passwd "$username" > /dev/null 2>&1; then
        bb-log-warning "User $username already exists"
    else
        # Check if group already exists
        if getent group "$groupname" > /dev/null 2>&1; then
            bb-log-warning "Group $groupname already exists"
        else
            # Check if group ID is already exists
            if getent group "$groupid" > /dev/null 2>&1; then
                bb-log-warning "Group ID $groupid is already exists"
                groupname=$(getent group "$groupid" | cut -d: -f1)
            else
                groupadd -g "$groupid" "$groupname"
            fi
        fi

        # Check if user ID is already exists
        if getent passwd "$userid" > /dev/null 2>&1; then
            bb-log-warning "User ID $userid is already exists"
        else
            useradd -m -u "$userid" -g "$groupname" "$username"
        fi
    fi
}
