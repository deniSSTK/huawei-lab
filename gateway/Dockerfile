FROM alpine:3.22

RUN apk add --no-cache nftables

COPY entrypoint.sh /usr/local/bin/gateway-entrypoint.sh

ENTRYPOINT ["/bin/sh", "/usr/local/bin/gateway-entrypoint.sh"]
