#!/bin/sh
# Sometimes nodes can be shown as Online without established connection to them.
# This is workaround for https://github.com/LINBIT/linstor-server/issues/331

# Collect list of satellite nodes
SATELLITES_ONLINE=$(linstor -m --output-version=v1 n l | jq -r '.[][] | select(.type == "SATELLITE" and .connection_status == "ONLINE").name' || true)
if [ -z "$SATELLITES_ONLINE" ]; then
  exit 0
fi

# Check online nodes with lost connection
linstor -m --output-version=v1 sp l -s DfltDisklessStorPool -n $SATELLITES_ONLINE | jq '.[][].reports[]?.message' | grep 'No active connection to satellite'
if [ $? -eq 0 ]; then
  exit 1
fi
