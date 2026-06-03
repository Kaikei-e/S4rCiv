# syntax=docker/dockerfile:1
FROM golang:1.26-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Generated protobuf/connect code is committed under gen/, so no buf at build time.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/collector ./cmd/collector

# Query side (read-only Connect-RPC server).
FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]

# Command side (collector: discover / poll / project).
FROM gcr.io/distroless/static-debian12:nonroot AS collector
WORKDIR /app
COPY --from=build /out/collector /app/collector
USER nonroot:nonroot
ENTRYPOINT ["/app/collector"]
