# syntax=docker/dockerfile:1.6

# ---- build stage ----
FROM golang:1.22-bookworm AS build
ARG BUILD_TAGS=""
ARG VERSION="dev"
WORKDIR /src

# Pre-cache go modules
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=$(if $(filter cgo_openssl,$(BUILD_TAGS)),1,0) \
    go build -trimpath -tags="${BUILD_TAGS}" \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o /out/tpm-crypto-service ./cmd/server

# ---- runtime stage ----
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=build /out/tpm-crypto-service /app/tpm-crypto-service
COPY --from=build /src/deploy/isolated/config/ /app/config/
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/app/tpm-crypto-service"]
CMD ["--config=/app/config/config.example.yaml"]
