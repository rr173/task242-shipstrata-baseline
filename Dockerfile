FROM golang:1.26.3-bookworm

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOFLAGS=-mod=mod
RUN go build -o /app/shipstrata ./cmd/shipstrata

EXPOSE 8080
ENTRYPOINT ["/app/shipstrata"]
CMD ["--smoke-test"]
