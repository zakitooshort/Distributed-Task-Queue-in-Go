FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /app/dashboard/dist

# build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

# server image
FROM alpine:3.19 AS server
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/server /server
# copy built dashboard if it exists
COPY --from=builder /app/dashboard/dist /dashboard/dist
EXPOSE 8080
CMD ["/server"]

# worker image
FROM alpine:3.19 AS worker
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/worker /worker
CMD ["/worker"]
