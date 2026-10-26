#!/bin/sh
set -eu

# Load the full policy before the gateway becomes healthy and Compose starts
# the clients. A configuration error stops the gateway instead of hiding it.
nft -f /etc/nftables.conf

exec sleep infinity
