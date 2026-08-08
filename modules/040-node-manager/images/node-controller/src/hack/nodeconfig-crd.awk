# Turns the controller-gen output for NodeConfig into the CRD the module ships:
# drops the controller-gen version annotation, adds the Deckhouse heritage
# labels every module CRD carries, and stamps x-kubernetes-sensitive-data onto
# status.maintenanceToken.
#
# That last marker is what makes the API strip the token from every
# get/list/watch answer that lacks the nodeconfigs/sensitive subresource, which
# nodes are deliberately not granted. controller-gen has no marker for it, so it
# is applied here. Fails loudly when the property is not found: a marker that
# silently stopped being applied publishes a root-equivalent credential to every
# node in the cluster.
#
# awk rather than a YAML editor, which would reindent the whole generated file.
#
# This should be pkg/crd-enricher instead: it addresses schema nodes by path
# rather than by counting indentation, so it cannot mis-target a marker or orphan
# a key, and it fails by construction when a path is missing. Migrating needs
# three markers on the Go types — deckhouse:sensitive-data on MaintenanceToken,
# kubebuilder:metadata:labels for the heritage labels, and any crd: marker to
# strip the version annotation — in a file this change does not own.
#
# One known cosmetic hole survives: a description block scalar that itself
# contains a `maintenanceToken:` line followed by an indented `type: string`
# would get a marker injected into the description text. The real property is
# still marked, and no controller-gen output looks like that.

BEGIN {
    # controller-gen emits YAML two spaces per level.
    nestingIndent = 2
}

# The metadata annotations block is buffered rather than dropped line by line:
# dropping the controller-gen line alone would leave an empty `annotations:` key,
# and dropping the key alone would orphan any other annotation controller-gen
# grows later — either way the output is invalid YAML under metadata:.
function flushAnnotations(   i) {
    if (!inAnnotations) {
        return
    }
    inAnnotations = 0
    if (kept == 0) {
        return
    }
    print "  annotations:"
    for (i = 1; i <= kept; i++) {
        print keptLines[i]
    }
}

/^  annotations:$/ {
    flushAnnotations()
    inAnnotations = 1
    kept = 0
    next
}
inAnnotations && /^    / {
    if ($0 ~ /^    controller-gen\.kubebuilder\.io\/version:/) {
        dropped = 1
        droppingValue = 1
        next
    }
    # Indented deeper than an annotation key means part of the value above it,
    # so a block scalar on the dropped annotation goes with its own key instead
    # of being orphaned under a bare `annotations:`.
    if (droppingValue && $0 ~ /^      /) {
        next
    }
    droppingValue = 0
    keptLines[++kept] = $0
    next
}
inAnnotations { droppingValue = 0; flushAnnotations() }

/^  name: nodeconfigs\.internal\.deckhouse\.io$/ {
    print
    print "  labels:"
    print "    heritage: deckhouse"
    print "    module: node-manager"
    named = 1
    next
}

{ print }

# The marker belongs to maintenanceToken's own schema, so it is stamped only on a
# `type: string` exactly one nesting level deeper than the property. Taking the
# first `type: string` that follows would, the day maintenanceToken stops being a
# bare string, silently move the marker onto whichever property comes next — and
# the CRD would ship publishing a root-equivalent credential to every node while
# the generator still reported success.
/^[[:space:]]*maintenanceToken:[[:space:]]*$/ {
    match($0, /^[[:space:]]*/)
    tokenIndent = RLENGTH
    inToken = 1
    next
}
inToken {
    # A blank line separates the paragraphs of the description block scalar; it
    # carries no indentation and so says nothing about where the schema ends.
    if ($0 ~ /^[[:space:]]*$/) {
        next
    }
    match($0, /^[[:space:]]*/)
    indent = RLENGTH
    if (indent <= tokenIndent) {
        # A sibling property: maintenanceToken's own schema is over, unmarked.
        inToken = 0
    } else if (indent == tokenIndent + nestingIndent && $0 ~ /^[[:space:]]*type: string[[:space:]]*$/) {
        print substr($0, 1, indent) "x-kubernetes-sensitive-data: true"
        inToken = 0
        marked = 1
    }
}

END {
    flushAnnotations()
    if (!dropped) {
        print "nodeconfig-crd: the controller-gen version annotation was not found in the generated CRD" > "/dev/stderr"
        exit 1
    }
    if (!named) {
        print "nodeconfig-crd: the CRD metadata.name was not found in the generated CRD" > "/dev/stderr"
        exit 1
    }
    if (!marked) {
        print "nodeconfig-crd: status.maintenanceToken was not found in the generated CRD" > "/dev/stderr"
        exit 1
    }
}
