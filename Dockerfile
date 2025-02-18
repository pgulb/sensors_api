FROM golang:1.24.0-alpine3.21 as builder

WORKDIR /src
COPY . .
RUN go build -o sensors_api .

FROM scratch

COPY --from=builder /src/sensors_api /app/sensors_api
CMD ["/app/sensors_api"]
