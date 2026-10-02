# Multi-stage build: static Go binary (CGO off — SQLite driver is pure Go) on distroless.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/terroir-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/terroir-api /terroir-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/terroir-api"]
