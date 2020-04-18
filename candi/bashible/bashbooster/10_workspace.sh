bb-var BB_WORKSPACE ".bb-workspace"

bb-workspace-init() {
    bb-log-debug "Initializing workspace at '$BB_WORKSPACE'"
    if [[ ! -d "$BB_WORKSPACE" ]]
    then
        mkdir -p "$BB_WORKSPACE" || bb-exit \
            $BB_ERROR_WORKSPACE_CREATION_FAILED \
            "Failed to initialize workspace at '$BB_WORKSPACE'"
    fi
    # Ensure BB_WORKSPACE stores absolute path
    BB_WORKSPACE="$( cd "$BB_WORKSPACE" ; pwd )"
}

bb-workspace-cleanup() {
    bb-log-debug "Cleaning up workspace at '$BB_WORKSPACE'"
    if [[ -z "$( ls "$BB_WORKSPACE" )" ]]
    then
        bb-log-debug "Workspace is empty. Removing"
        rm -rf "$BB_WORKSPACE"
    else
        bb-log-debug "Workspace is not empty"
    fi
}
