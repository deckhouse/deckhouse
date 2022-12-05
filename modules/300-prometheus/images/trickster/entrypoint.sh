#!/bin/sh

cat "/etc/trickster/trickster.conf" | envsubst > /tmp/trickster.conf
exec trickster
