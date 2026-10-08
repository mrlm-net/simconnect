# The traffic director (#710) as a container: it needs no simulator. Built
# from the repository root:
#
#	docker build -f cmd/traffic-director/Dockerfile -t traffic-director .
#	docker run -p 7710:7710 -p 8080:8080 traffic-director \
#	  -listen :7710 -jwks https://mycrew.outlays.dev/v1/traffic/jwks \
#	  -jwks-iss … -jwks-aud … -tls-cert /certs/tls.crt -tls-key /certs/tls.key
#
# The actuators dial in on -listen (TLS with -tls-cert/-tls-key); the HTTP
# API is on -addr (here 0.0.0.0:8080), /healthz on it without a token.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/traffic-director ./cmd/traffic-director

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 director && mkdir -p /data && chown director /data
COPY --from=build /out/traffic-director /usr/local/bin/traffic-director
USER director
WORKDIR /data
EXPOSE 7710 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["traffic-director", "-addr", "0.0.0.0:8080", "-log-dir", "/data", "-data-dir", "/data"]
