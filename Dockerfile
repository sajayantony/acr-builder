FROM golang:1.9.1-stretch as build
RUN go get -u github.com/kisielk/errcheck &&\
    go get -u honnef.co/go/tools/cmd/megacheck &&\
    go get -u github.com/golang/lint/golint

WORKDIR /go/src/github.com/Azure/acr-builder
COPY ./ /go/src/github.com/Azure/acr-builder
RUN echo "Running Static Analysis tools..." &&\
    echo "Running GoVet..." &&\
    go vet $(go list ./... | grep -v /vendor/) &&\
    echo "Running ErrCheck..." &&\
    errcheck $(go list ./... | grep -v /vendor/) &&\
    echo "Running MegaCheck..." &&\
    megacheck $(go list ./... | grep -v /vendor/) &&\
    echo "Running golint..." &&\
    golint -set_exit_status $(go list ./... | grep -v '/vendor/' | grep -v '/tests/') &&\
    echo "Running tests..." &&\
    go test -cover $(go list ./... | grep -v /vendor/ | grep -v '/tests/') &&\
    echo "Verification successful, building binaries..." &&\
    GOOS=linux GOARCH=386 go build

FROM docker:17.12.0-ce-git
RUN apk add --update --no-cache \
    openssh \
    openssl \
    ca-certificates \
    python \
    py-pip \
    build-base \
    && pip install docker-compose \
    && rm -rf /var/cache/apk/*
COPY --from=build /go/src/github.com/Azure/acr-builder/acr-builder /usr/local/bin
ENTRYPOINT ["/usr/local/bin/acr-builder"]
CMD []
