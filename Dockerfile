FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/static /app/static
EXPOSE 8000
CMD ["/app/server"]
