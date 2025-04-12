FROM golang AS builder
RUN apt-get update && apt install -y protobuf-compiler
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN make setup gen
RUN CGO_ENABLED=0 GOOS=linux go build -o /main cmd/main.go

FROM alpine
WORKDIR /
COPY --from=builder /main /main
EXPOSE 3000
ENTRYPOINT ["/main"]
