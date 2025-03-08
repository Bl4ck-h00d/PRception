FROM golang:1.23 AS builder

WORKDIR /app

COPY . .

# Install dependencies and build the app
RUN go mod download
RUN go build -o prception ./cmd/main.go

FROM gcr.io/distroless/base

WORKDIR /

COPY --from=builder /app/prception .
COPY --from=builder /app/.env . 

# Expose port 8080
EXPOSE 8080

# Start the server
CMD [ "./prception" ]
