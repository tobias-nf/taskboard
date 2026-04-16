FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /taskboard-api ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /taskboard-api /taskboard-api
ENV ENV=development
EXPOSE 4000
CMD ["/taskboard-api"]
