#!/bin/sh
set -eu

# Load the full policy before the gateway becomes healthy and Compose starts
# the clients. A configuration error stops the gateway instead of hiding it.
uplink_interface=$(ip -o -4 addr show | awk '$4 == "172.28.40.254/24" { print $2 }')
if [ -z "$uplink_interface" ]; then
    printf 'Missing gateway uplink interface\n' >&2
    exit 1
fi

# Docker chooses the uplink default route through gw_priority. Resolve the
# interface by its assigned IP rather than assuming Docker interface order.
# Both tables are loaded in a single nftables transaction.
printf 'include "/etc/nftables.conf"\ninclude "/etc/nat.conf"\n' |
    nft --define "uplink_if=\"$uplink_interface\"" -f -

exec sleep infinity
