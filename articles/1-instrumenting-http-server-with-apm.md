# Go: Instrumenting the HTTP service with Elastic APM

Elastic APM is an application performance monitoring product Elastic, the makers of
Elasticsearch, and most notably the ELK stack (Elasticsearch, Logstash, Kibana). APM is another
plug and play product of theirs, that hooks up to your existing ELK installation and
provides endpoints for application agents to receive performance metrics data.

Elastic provides a Go agent, which we will use to instrument various parts of our code.
The instrumentation will help us not only with performance metrics, but with debugging
and optimization of our applications as well.

We want to start by logging our incoming requests. For that we can just modify our generator
for `main.go`, and add the very simple wrapper to our HTTP handler.

~~~diff
--- a/templates/cmd_main.go.tpl
+++ b/templates/cmd_main.go.tpl
@@ -10,6 +10,7 @@ import (
        "net/http"

        _ "github.com/go-sql-driver/mysql"
+       "go.elastic.co/apm/module/apmhttp"

        "${MODULE}/rpc/${SERVICE}"
        server "${MODULE}/server/${SERVICE}"
@@ -25,5 +26,5 @@ func main() {

        twirpHandler := ${SERVICE}.New${SERVICE_CAMEL}ServiceServer(srv, nil)

-       http.ListenAndServe(":3000", twirpHandler)
+       http.ListenAndServe(":3000", apmhttp.Wrap(twirpHandler))
 }
~~~

The wrapper takes care of creating what is called a "Transaction". A transaction in our
case consists of the incoming request to our handler, all the way until the completion of
that request. For each requests, various data is logged to Elastic APM, the most obvious
one of which is the request duration.

Any request coming to your service may error out. When creating twirpHandler, we omit
the second parameter to the `NewXXXServiceServer` call, a parameter of `*twirp.ServerHooks`
type. This structure defines various hooks implemented by the Twirp RPC server handler.
We are especially interested in the `Error` hook:

~~~go
type ServerHooks struct {
	// Error hook is called when an error occurs while handling a request. The
	// Error is passed as argument to the hook.
	Error func(context.Context, Error) context.Context
}
~~~

Particularly, we want to log the error with another Elastic APM API call, CaptureError.
For that we need to modify our main.go generator a bit more:

~~~diff
--- a/templates/cmd_main.go.tpl
+++ b/templates/cmd_main.go.tpl
@@ -10,6 +10,8 @@ import (
        "net/http"

        _ "github.com/go-sql-driver/mysql"
+       "github.com/twitchtv/twirp"
+       "go.elastic.co/apm"
        "go.elastic.co/apm/module/apmhttp"

        "${MODULE}/rpc/${SERVICE}"
@@ -24,7 +26,12 @@ func main() {
                log.Fatalf("Error in service.New(): %+v", err)
        }

-       twirpHandler := ${SERVICE}.New${SERVICE_CAMEL}ServiceServer(srv, nil)
+       twirpHandler := ${SERVICE}.New${SERVICE_CAMEL}ServiceServer(srv, &twirp.ServerHooks{
+               Error: func(ctx context.Context, err twirp.Error) context.Context {
+                       apm.CaptureError(ctx, err).Send()
+                       return ctx
+               },
+       })
~~~

We add the relevant imports, to put together the Error hook. The interface for twirp.Error is
a superset of error, so it is already compatible with the parameter for apm.CaptureError.