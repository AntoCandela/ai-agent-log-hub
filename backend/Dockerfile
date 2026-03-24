FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /loghub ./cmd/loghub

FROM alpine:3.19
COPY --from=builder /loghub /usr/local/bin/loghub
COPY migrations/ /migrations/
EXPOSE 4800
CMD ["loghub"]
