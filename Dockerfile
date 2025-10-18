# build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'main.AppName=clicks-api'" -o /out/clicks-api ./cmd/clicks-api

# runtime (distroless)
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/clicks-api /app/clicks-api
EXPOSE 3000
USER 65532:65532
ENTRYPOINT ["/app/clicks-api"]
