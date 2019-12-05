# Go: Introduction to Protobuf: GRPC

We implemented a Twitch RPC generator in the previous chapter, but we
do have some hard core GRPC fans that requested that I should check out
what GRPC produces. I am interested into comparing these two as well,
so let's start by scaffolding our GRPC handlers. Our main goal is to compare
the implementation details of GRPC over Twitch RPC in more concrete terms.

## Generating the GRPC scaffolding

We need to modify the `Makefile` slightly, in order to include the `grpc`
codegen plugin, by changing a protoc option:

Under the `rpc.%` target, change the `go_out` option, from:

~~~text
--go_out=paths=source_relative:.
~~~

to the following:

~~~
--go_out=plugins=grpc,paths=source_relative:.
~~~

This will include the `grpc` plugin that generates the code for the GRPC
Client and Server. The first thing we can notice is that there is a significant
change in our `go.sum` file, namely that 42 different package versions have
been listed as dependencies. The only package that really stands out however
is `prometheus/client_model`. This might mean that the GRPC server implementation
internals support some custom prometheus metrics out of the box. We definitely
don't get that from Twitch RPC, but we are planning to add on instrumentation.

## Comparing the server side

Inspecting the changed `*.pb.go` files, we can compare the interface produced
by Twitch RPC, and what GRPC produces. The GRPC protoc generator produces two
distinct interfaces, `StatsServiceClient` and `StatsServiceServer`. As we are
interested in the first one, let's compare it now:

~~~go
// StatsServiceServer is the server API for StatsService service.
type StatsServiceServer interface {
	Push(context.Context, *PushRequest) (*PushResponse, error)
}
~~~

Compared to Twitch RPC:

~~~go
type StatsService interface {
	Push(context.Context, *PushRequest) (*PushResponse, error)
}
~~~

So, first, we see that the implementation for our Twitch RPC service is
compatible with our GRPC server. This means that our workflow will not
change a single bit, if we decide to migrate from Twitch to GRPC.

We can run a single service, which exposes both GRPC and Twirp endpoints.
Maintaining them both seems like a bad idea, but as we don't have
any Twirp specific implementation in our service itself, it seems
like we can manage to run both endpoints without difficulty.

## Comparing the client side

The difference seems to be in the client itself:

~~~go
type StatsServiceClient interface {
	Push(ctx context.Context, in *PushRequest, opts ...grpc.CallOption) (*PushResponse, error)
}
~~~

While the Twirp Client and Server conform to a single interface, GRPC
clients have additional options available. A cursory reading of [grpc.CallOption](https://godoc.org/google.golang.org/grpc#CallOption)
gives us a list of possible constructors. As is supposedly common with Google
APIs, of the listed options currently:

- CallContentSubtype (string) can be set to use JSON encoding on the wire,
- CallCustomCodec - DEPRECATED (use ForceCodec)
- FailFast - DEPRECATED (use WaitForReady)
- ForceCodec - EXPERIMENTAL API (wait, we just came here from CallCustomCodec)
- Header - retrieves header metadata
- MaxCall(Recv/Send)MsgSize - client message size limits
- MaxRetryRPCBufferSize - EXPERIMENTAL
- Peer (p *peer.Peer) - Populate *p after RPC completes
- PerRPCCredentials - Sets credentials for a RPC call
- Trailer (md *metadata.MD) - returns trailer metadata (no idea what is a Trailer)
- UseCompressor - EXPERIMENTAL
- WaitForReady (waitForReady bool) - if false, fail immediately, if true block/retry (default=false)

So, to summarize - out of all those options, 3 are EXPERIMENTAL, 2 are DEPRECATED and one of
them is pointing to an EXPERIMENTAL option, and the biggest question raised is how the GRPC
client behaves in relation to WaitForReady, and which option is recommended if any.

In comparison, Twitch RPC produces, at least in my opinion, a nicer interface for specific clients:

~~~go
// NewStatsServiceProtobufClient creates a Protobuf client that implements the StatsService interface.
func NewStatsServiceProtobufClient(addr string, client HTTPClient) StatsService {
...

// NewStatsServiceJSONClient creates a JSON client that implements the StatsService interface.
func NewStatsServiceJSONClient(addr string, client HTTPClient) StatsService {
...
~~~

GRPC could benefit from some interface deduplication here. The GRPC clients constructor could
take those particular call options so both the client and server could conform to the same interface.
Also, much as the server side implementation, it's also obvious that GRPC implements the client
side as well, as the transport isn't based on net/http:

~~~go
// NewStatsServiceClient creates a GRPC client that implements the StatsServiceClient interface.
func NewStatsServiceClient(cc *grpc.ClientConn) StatsServiceClient {
...
~~~

What we can also see is that the GRPC client can authenticate to the GRPC server via
the PerRPCCredentials option. This is also something that Twitch RPC doesn't provide for us.
We don't need it for our service, but it's something to consider if you want to increase
the level of security inside your service mesh.

We won't create the GRPC server just yet, but we'll keep this codegen option enabled for the future.
As we already discussed, GRPC is a great framework to have when we have clients that can speak
it natively. It's not great for the browser or the javascript console (am I wrong?), and it's definitely
not great for debugging/sniffing traffic, but using the proto files to generate the clients for
Android/iPhone apps is a valid use case. GRPC has wider code generation support than Twirp and
is a better option if you want to cover a larger ecosystem.