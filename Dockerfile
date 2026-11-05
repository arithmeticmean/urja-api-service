# Build stage: compile the service and the goose migration tool.
FROM golang:1.27-alpine AS build
ENV CGO_ENABLED=0
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

COPY . .
RUN go build -o /out/urja-api-service .

# Run stage: a small image with just the two binaries and the migrations.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/urja-api-service /usr/local/bin/urja-api-service
COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY migrations /migrations

EXPOSE 8080
CMD ["urja-api-service"]
