FROM golang:1.13-stretch
ENV CGO_ENABLED=0

WORKDIR /go/src/cfunc
COPY cfunc/go.mod .
COPY cfunc/go.sum .
RUN go mod download
COPY cfunc/*.go ./

WORKDIR /go/src/config-function/sub1
COPY consul/acl-bootstrap/go.mod .
COPY consul/acl-bootstrap/go.sum .
RUN go mod download
COPY consul/acl-bootstrap/*.go ./
RUN go build -v -o /usr/local/bin/config-function ./

FROM alpine:latest
COPY --from=0 /usr/local/bin/config-function /usr/local/bin/config-function
CMD ["config-function"]
