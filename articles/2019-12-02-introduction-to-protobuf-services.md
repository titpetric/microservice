# Go: Introduction to Protobuf: Services

The next step (or possibly the first step) about implementing a microservice, is defining
it's API endpoints. What people usually do is write http handlers and resort to a routing
framework like [go-chi/chi](https://github.com/go-chi/chi). But, protocol buffers can define
your service, as RPC calls definitions are built into the protobuf schema.

## Protobuf service definitions

Protobuf files may define a `service` object, which has definitions for one or many `rpc` calls.
Let's extend our proto definitions for `stats.proto`, by including a service with a defined RPC.

~~~protobuf
service StatsService {
	rpc Push(PushRequest) returns (PushResponse);
}
~~~

A RPC method is defined with the `rpc` keyword. The service definition here declares a RPC named `Push`,
which takes a message `PushRequest` as it's input, and returns the message `PushResponse` as it's output.
All RPC definitions should follow the same naming pattern as a best practice. This way you can extend
PushRequest and PushResponse, without introducing breaking changes to the RPC definition as well.

Updating the `.proto` file definitions in our project by default doesn't change anything. We need to generate
a RPC scaffold using a RPC framework. For Go RPCs, we can consider GRPC from the beginning, or we can look
towards a more simple [Twitch RPC aka. Twirp](https://github.com/twitchtv/twirp).

Common reasons for choosing Twirp over GRPC are as follows:

- Twirp comes with HTTP 1.1 support (vendors need to catch up to HTTP/2 still),
- Twirp supports JSON transport out of the gate,
- GRPC re-implements HTTP/2 outside of net/http,

And reasons for GRPC over Twirp are:

- GRPC supports streaming (Twirp has [an open proposal for streaming APIs](https://github.com/twitchtv/twirp/issues/70))
- GRPC makes wire compatibility promises (this is built in to protobufs)
- More functionality on the networking level (retries, rpc discovery,...)

JSON would be the preferable format to demonstrate payloads, especially in terms of inspecting/sharing the payloads
in documentation and similar. While GRPC is written by Google, there are many tools that you'd have to add in to make
it a bit more developer friendly - [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) to add HTTP/1.1,
and [grpcurl](https://github.com/fullstorydev/grpcurl) to issue json-protobuf bridged requests.

GRPC is a much more rich RPC framework, supporting a wider array of use cases. If you feel that you need RPC streaming
or API discovery, or your use cases lay beyond a simple request/response model, GRPC might be your only option.

## Our microservice

We'll be dealing with Twitch RPC from here on out, since it serves our requirements.

About 10 years back I wrote a relatively simple microservice that is basically just tracking news item views. That
solution is proving to be unmaintainable 10 years later at best, but still pretty good so it manages 0-1ms/request.
It's also a bit smarter than that, since it tracks a number of assets that aren't news in the same way. So, effectively
the service is tracking views in a multi-tennant way, for a predefined set of applications.

Let's refresh what our current service definition is:

~~~protobuf
service StatsService {
	rpc Push(PushRequest) returns (PushResponse);
}

message PushRequest {
	string property = 1;
	uint32 section = 2;
	uint32 id = 3;
}

message PushResponse {}
~~~

Our `StatsService` defines a RPC called `Push`, which takes a message with three parameters:

- property: the key name for a tracked property, e.g. "news"
- section: a related section ID for this property (numeric)
- id: the ID which defines the content being viewed (numeric)

The goal of the service is to log the data in PushRequest, and aggregate it over several time periods.
The aggregation itself is needed to provide data sets like "Most read news over the last 6 months".

## Twitch RPC scaffolding

The main client and server code generators for Twitch RPC are listed in the README for [twitchtv/twirp](https://github.com/twitchtv/twirp).
The code generator we will use is available from `github.com/twitchtv/twirp/protoc-gen-twirp`. We will add this to our dockerfile:

~~~diff
--- a/docker/build/Dockerfile
+++ b/docker/build/Dockerfile
@@ -21,3 +21,6 @@ RUN go get -u google.golang.org/grpc

 # Install protoc-gen-go
 RUN go get -u github.com/golang/protobuf/protoc-gen-go
+
+# Install protoc-gen-twirp
+RUN go get github.com/twitchtv/twirp/protoc-gen-twirp
~~~

And now we can extend our code generator in the `.drone.yml` file, by generating the twirp RPC output as well:

~~~diff
--- a/.drone.yml
+++ b/.drone.yml
@@ -10,6 +10,7 @@ steps:
   pull: always
   commands:
     - protoc --proto_path=$GOPATH/src:. -Irpc/stats --go_out=paths=source_relative:. rpc/stats/stats.proto
+    - protoc --proto_path=$GOPATH/src:. -Irpc/stats --twirp_out=paths=source_relative:. rpc/stats/stats.proto
~~~

We run the `protoc` command twice, but the `--twirp_out` option could actually be added to the existing command.
We will keep this seperate just to help with readability, so we know which command is responsible to generate what.
When it comes to the code generator plugins for protoc, there's a long list of plugins that can generate anything
from JavaScript clients to Swagger documentation. As we will add these, we don't want the specific for generating
one type of output to bleed into other generator rules.

The above command will generate a `stats.twirp.go` file in the same folder as `stats.proto` file. The important
part for our implementation is the following interface:

~~~go
type StatsService interface {
        Push(context.Context, *PushRequest) (*PushResponse, error)
}
~~~

In order to implement our Twitch RPC service, we need an implementation for this interface. For that, we will
look at our own code generation that will help us with this. Particularly, we want to scaffold both the server
and the client code that could get updated if our service definitions change.