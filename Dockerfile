FROM golang:1.27.1 AS build

WORKDIR /src
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY api/ ./api/
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/trip-service ./cmd/trip-service

FROM scratch
COPY --from=build /out/trip-service /trip-service
USER 65532:65532
ENTRYPOINT ["/trip-service"]
