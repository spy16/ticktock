FROM golang:1.20-alpine AS builder
RUN apk --no-cache add ca-certificates make
WORKDIR /app
COPY . .
RUN make

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/ticktock .

### Your code will be inserted here ###

CMD ["/app/ticktock", "serve", "--addr=0.0.0.0:8080", "--pprof"]
