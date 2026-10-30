FROM golang:1.26-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY auth/go.mod auth/go.sum ./
RUN go mod download
COPY auth/ ./
RUN CGO_ENABLED=1 go build -trimpath -o /portal ./cmd/portal

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S portal && adduser -S -G portal portal && mkdir /data && chown portal:portal /data
COPY --from=build /portal /usr/local/bin/portal
USER portal
ENTRYPOINT ["/usr/local/bin/portal"]
