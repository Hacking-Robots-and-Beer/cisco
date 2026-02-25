FROM golang:1.23-alpine AS builder

WORKDIR /app

# Allow go to fetch and resolve modules (generates go.sum if not present)
ENV GONOSUMDB=*
ENV GOFLAGS=-mod=mod

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /cisco-api ./cmd/main.go

# ---

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /cisco-api .

EXPOSE 8080

CMD ["/app/cisco-api"]
