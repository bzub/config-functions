FROM golang:1.14-stretch
ARG config_function
ENV CGO_ENABLED=0
WORKDIR /go/src/config-functions
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . ./
RUN go build -v -o /usr/local/bin/config-function "./${config_function}/cmd/config-function"

FROM alpine:latest
COPY --from=0 /usr/local/bin/config-function /usr/local/bin/config-function
CMD ["config-function"]
