FROM golang:1.26-alpine AS build
WORKDIR /src
COPY auth/go.mod auth/go.sum ./
COPY auth/internal/control ./internal/control
COPY auth/cmd/gateway ./cmd/gateway
RUN CGO_ENABLED=0 go build -trimpath -o /gateway-control ./cmd/gateway

FROM alpine:3.22
RUN apk add --no-cache nftables
COPY --from=build /gateway-control /usr/local/bin/gateway-control
COPY gateway/entrypoint.sh /usr/local/bin/gateway-entrypoint.sh
ENTRYPOINT ["/bin/sh", "/usr/local/bin/gateway-entrypoint.sh"]
