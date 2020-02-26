FROM golang:1.13

# install protobuf
ENV PB_VER 3.10.1
ENV PB_URL https://github.com/google/protobuf/releases/download/v${PB_VER}/protoc-${PB_VER}-linux-x86_64.zip

RUN apt-get -qq update && apt-get -qqy install curl git make unzip gettext rsync

RUN mkdir -p /tmp/protoc && \
    curl -L ${PB_URL} > /tmp/protoc/protoc.zip && \
    cd /tmp/protoc && \
    unzip protoc.zip && \
    cp /tmp/protoc/bin/protoc /usr/local/bin && \
    cp -R /tmp/protoc/include/* /usr/local/include && \
    chmod go+rx /usr/local/bin/protoc && \
    cd /tmp && \
    rm -r /tmp/protoc

# Get the source from GitHub
RUN go get -u google.golang.org/grpc

# Install protoc-gen-go
RUN go get -u github.com/golang/protobuf/protoc-gen-go

# Install protoc-gen-twirp
RUN go get -u github.com/twitchtv/twirp/protoc-gen-twirp

# Install interface scaffolder
RUN go get -u github.com/josharian/impl

# Install google wire for DI
RUN go get -u github.com/google/wire/cmd/wire

# Additional twirp outputs
RUN go get -u github.com/thechriswalker/protoc-gen-twirp_js
RUN go get -u github.com/elliots/protoc-gen-twirp_swagger

# GolangCI lint
RUN cd /usr/local && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.23.1
