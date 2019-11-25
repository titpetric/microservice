# Go: Introduction to Protobuf: Messages

In it's most basic ways, protobufs are an approach to serialize structured data
with minimal overhead. They require that the data structures be known and compatible
between the client and server, unlike JSON where the structure itself is part of
the encoding.

## Protobufs and Go

The most basic protobuf message definition would look something like this:

~~~proto
message ListThreadRequest {
	// session info
	string sessionID = 1;

	// pagination
	uint32 pageNumber = 2;
	uint32 pageSize = 3;
}
~~~

The above message structure specifies the field names, types, and it's order in the
encoded binary structure. Managing the structure has a few requirements that mean
different things, if the structures are used as protobufs, or as JSON encoded data.

For example, this is the `protoc` generated code for this message:

~~~go
type ListThreadRequest struct {
	// session info
	SessionID string `protobuf:"bytes,1,opt,name=sessionID,proto3" json:"sessionID,omitempty"`
	// pagination
	PageNumber           uint32   `protobuf:"varint,2,opt,name=pageNumber,proto3" json:"pageNumber,omitempty"`
	PageSize             uint32   `protobuf:"varint,3,opt,name=pageSize,proto3" json:"pageSize,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}
~~~

In the development process, managing the protobuf messages in a forward compatible way,
there are some rules to follow:

- Adding new fields is not a breaking change
- Removing fields is not a breaking change
	- Don't reuse a field number (breaks existing protobuf clients),
	- Don't re-number fields (breaks existing protobuf clients),
- Renaming fields is not a breaking change for protobuf
	- But will break JSON encoding where schema is encoded into JSON
- Changing field types is a breaking change for protobuf, mostly JSON as well
	- Numeric types e.g. uint16 to uint32 are generally safe for JSON

So, basically, if you're relying on changing the protobuf message definitions during your development process,
you'll need to keep the client(s) up to date with the server. This is especially important later, if you use
the protobuf API with mobile clients (Android and iPhone apps), since making breaking changes to the API is
going to hurt you there. Adding new fields or deleting old fields is the safest way to make changes to the API,
as the protobufs definitions stay compatible.

## Generating Protobuf Go code

In this series of articles, I'm going to build out a real world microservice, that tracks and aggregates
view data for a number of services. It doesn't need authentication and is in it's definition a true microservice,
as it will only need a single API endpoint.

We will create our `stats` microservice, by creating a `rpc/stats/stats.proto` file to begin with.

~~~proto
syntax = "proto3";

package stats;

option go_package = "github.com/titpetric/microservice/rpc/stats";

message PushRequest {
	string property = 1;
	uint32 section = 2;
	uint32 id = 3;
}

message PushResponse {}
~~~

Here a `proto3` version is declared. The important parts are the `go_package` option: with this an import
path is defined for our service, which is useful if another service wants to import and use the message
definitions here. Reusability is a protobuf built-in feature.

Since we don't want to do things half-way, we're going to approach our microservice with a CI-first approach.
Using Drone CI is a great option for using a CI from the beginning, as it's [drone/drone-cli](https://github.com/drone/drone-cli)
doesn't need a CI service set up, and you can just run the CI steps locally by running `drone exec`.

In order to set up out microservice build framework, we need:

1. Drone CI drone-cli installed
2. A docker environment with `protoc` and `protoc-gen-go` installed,
4. A `Makefile` to help us out for the long run
3. A `.drone.yml` config files with the build steps for generating go code,

### Installing Drone CI

Installing drone-cli is very simple. You can run the following if you're on an amd64 linux host, otherwise
just visit the [drone/drone-cli](https://github.com/drone/drone-cli) releases page and pull the version
relevant for you and unpack it into `/usr/local/bin` or your common executable path.

~~~
cd /usr/local/bin
wget https://github.com/drone/drone-cli/releases/download/v1.2.0/drone_linux_amd64.tar.gz
tar -zxvf drone*.tar.gz && rm drone*.tar.gz
~~~~

### Creating a build environment

Drone CI works by running CI steps you declare in `.drone.yml` in your provided Docker environment. For our build
environment, I've created `docker/build/`, and inside a `Dockerfile` and a `Makefile` to assist with building and
publishing the build image required for our case:

~~~Dockerfile
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
~~~

And the `Makefile`, implementing `make && make push` to quickly build and push our image to the docker registry.
The image is published under `titpetric/microservice-build`, but I suggest you manage your own image here.

~~~Makefile
.PHONY: all docker push test

IMAGE := titpetric/microservice-build

all: docker

docker:
	docker build --rm -t $(IMAGE) .

push:
	docker push $(IMAGE)

test:
	docker run -it --rm $(IMAGE) sh
~~~

### Creating a Makefile helper

It's very easy to run `drone exec`, but our requirements will grow over time and the Drone CI steps
will become more complex and harder to manage. Using a Makefile enables us to add more complex targets
which we will run from Drone with time. Currently we can start with a minimal Makefile which just
wraps a call to `drone exec`:

```Makefile
.PHONY: all

all:
	drone exec
```

This very simple Makefile means that we'll be able to build our project with Drone CI at any time just by running `make`.
We will extend it over time to support new requirements, but for now we'll just make sure it's available to us.

### Creating a Drone CI config

With this, we can define our initial `.drone.yml` file that will build our Protobuf struct definitions, as
well as perform some maintenance on our codebase:

~~~
workspace:
  base: /microservice

kind: pipeline
name: build

steps:
- name: test
  image: titpetric/microservice-build
  pull: always
  commands:
    - protoc --proto_path=$GOPATH/src:. -Irpc/stats --go_out=paths=source_relative:. rpc/stats/stats.proto
    - go mod tidy > /dev/null 2>&1
    - go mod download > /dev/null 2>&1
    - go fmt ./... > /dev/null 2>&1
~~~

The housekeeping done is for our go.mod/go.sum files, as well as running `go fmt` on our codebase.

The first step defined under the `commands:` is our `protoc` command that will generate the Go definitions
for our declared messages. In the folder where our `stats.proto` file lives, a `stats.pb.go` file will
be created, with structures for each declared `message {}`.

## Wrapping up

So, what we managed to achieve here:

- we created our CI build image with our `protoc` code generation environment,
- we are using Drone CI as our local build service, enabling us to migrate to a hosted CI in the future,
- we created a protobuf definition for our microservice message structures,
- we generated the appropriate Go code for encoding/decoding the protobuf messages

From here on out, we will move towards implementing a RPC service.