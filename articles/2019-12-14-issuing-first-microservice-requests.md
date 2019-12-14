# Issuing our first microservice requests

In order to test our microservice, we require a database, running migrations,
and our service itself. We're going to build a docker-compose.yml file which
takes care of all of this.

## The docker-compose file

In order to use the docker-compose.yml file, first you need to run
`make && make docker` to build the required docker images.

I started with a simple docker-compose.yml file, like this:

~~~yaml
version: '3.4'

services:
  stats:
    image: titpetric/service-stats
    restart: always
    environment:
      DB_DSN: "stats:stats@tcp(db:3306)/stats"

  migrations:
    image: titpetric/service-db-migrate-cli
    command: [
      "-db-dsn=stats:stats@tcp(db:3306)/stats",
      "-service=stats",
      "-real=true"
    ]

  db:
    image: percona/percona-server:8.0.17
    environment:
      MYSQL_ALLOW_EMPTY_PASSWORD: "true"
      MYSQL_USER: "stats"
      MYSQL_DATABASE: "stats"
      MYSQL_PASSWORD: "stats"
    restart: always
~~~

Running a microservice requires some coordination between containers,
which we will do by hand, because:

- migrations aren't part of our service,
- we don't have database reconnect logic yet so migrations
  don't wait for the database to become available

We will fix that, but for now lets do some of the manual legwork
in order to make this work. Issue the following commands:

~~~bash
# first run the database
docker-compose up -d db
# wait for a bit so it gets provisioned
sleep 30
# run migrations
docker-compose run --rm migrations
# run our service
docker-compose up -d
~~~

Now, generally anytime you'd just do `docker-compose up -d` should bring
up our service, database, and update the migrations (leaving a container behind
since docker-compose doesn't have a `rm: true` option we could use here).
This is why we should update the migrations to be run from our service with
a particular command line parameter or environment flag. We'll come back to that.

If you did everything correctly, `docker-compose ps` should output something like:

~~~plaintext
        Name                     Command              State          Ports       
---------------------------------------------------------------------------------
microservice_db_1      /docker-entrypoint.sh mysqld   Up      3306/tcp, 33060/tcp
microservice_stats_1   /app/service                   Up      3000/tcp           
~~~

## Our first request

Now, we didn't define any exposed ports on the container, but that's not important.
We can access any publicly exposed port from the container, all we need to do is
find out the IP that it's using. We can do that like this:

~~~plaintext
# docker inspect microservice_stats_1 | grep IPAddress
      "SecondaryIPAddresses": null,
      "IPAddress": "",
              "IPAddress": "172.24.0.3",
~~~

We can quickly verify by issuing a simple curl request:

~~~json
# curl -s http://172.24.0.3:3000 | jq .
{
  "code": "bad_route",
  "msg": "unsupported method \"GET\" (only POST is allowed)",
  "meta": {
    "twirp_invalid_route": "GET /"
  }
}
~~~

The actual endpoint for our service is available on the following link:

- `/twitch/stats.StatsService/Push`

Let's first start with bogus request with invalid data:

~~~bash
curl -s -X POST -H 'Content-Type: application/json' \
     http://172.24.0.3:3000/twirp/stats.StatsService/Push \
     -d '{"userID": "2"}' | jq .
~~~

The response we get back is:

~~~json
{
  "code": "internal",
  "msg": "received a nil *PushResponse and nil error while calling Push. nil responses are not supported"
}
~~~

Well, the error isn't entirely expected, but it's an easy fix. Instead of returning nil at
the end of our Push function, we'll just create a PushResponse instance with `new()`:

~~~diff
--- a/server/stats/server_push.go
+++ b/server/stats/server_push.go
@@ -31,5 +31,5 @@ func (svc *Server) Push(ctx context.Context, r *stats.PushRequest) (*stats.PushR
 
        query := fmt.Sprintf("insert into %s (%s) values (%s)", IncomingTable, fields, named)
        _, err = svc.db.NamedExecContext(ctx, query, row)
-       return nil, err
+       return new(stats.PushResponse), err
 }
~~~

Re-issuing the requests returns a valid but empty response (JSON: `{}`). But wait, we literally
sent invalid data to our RPC, how come we didn't error out? Let's revisit the structure for
`PushRequest`:

~~~go
type PushRequest struct {
	Property string `json:"property,omitempty"`
	Section  uint32 `json:"section,omitempty"`
	Id       uint32 `json:"id,omitempty"`
}
~~~

Inspecting the JSON tags on the PB generated fields, we see that they have `omitempty` set. This
is the reason why we don't get a JSON decoder error on the request, as none of the fields are mandatory.
The encoding/json package also works in a way, where you don't need to decode every bit of the JSON
structure, so our bogus `userID` payload just gets ignored.

Obviously, this a job for validation. Let's add some basic checks for valid input. Add the following
checks at the beginning of our Push() implementation:

~~~go
	validate := func() error {
		if r.Property == "" {
			return errors.New("Missing Property")
		}
		if r.Property != "news" {
			return errors.New("Invalid Property")
		}
		if r.Id < 1 {
			return errors.New("Missing ID")
		}
		if r.Section < 1 {
			return errors.New("Missing Section")
		}
		return nil
	}
	if err := validate(); err != nil {
		return nil, err
	}
~~~

Rebuild with `make build`, `make docker.stats` and reload the service with `docker-compose up -d`.
Retrying the previous request now ends up like this:

~~~json
{
  "code": "internal",
  "msg": "Missing Property",
  "meta": {
    "cause": "*errors.errorString"
  }
}
~~~

Yay! Let's craft a valid request, and get back to an empty response.

~~~bash
#!/bin/bash
payload='{
  "property": "news",
  "section": 1,
  "id": 1
}'

curl -s -X POST -H 'Content-Type: application/json' \
     http://172.24.0.3:3000/twirp/stats.StatsService/Push \
     -d "$payload" | jq .
~~~

Everything works as expected. Let's verify by inspecting the contents of the incoming
table in the database. Run the following command to verify:

~~~plaintext
# docker-compose exec db mysql -u root stats -e 'select * from incoming'
+--------------------+----------+------------------+-------------+------------+---------------------+
| id                 | property | property_section | property_id | remote_ip  | stamp               |
+--------------------+----------+------------------+-------------+------------+---------------------+
| 279831112371404803 |          |                0 |           0 | 172.24.0.1 | 2019-12-14 11:12:21 |
| 279831611778793475 |          |                0 |           0 | 172.24.0.1 | 2019-12-14 11:17:18 |
| 279833914468466691 |          |                0 |           0 | 172.24.0.1 | 2019-12-14 11:40:11 |
| 279835838647369731 | news     |                1 |           1 | 172.24.0.1 | 2019-12-14 11:59:18 |
+--------------------+----------+------------------+-------------+------------+---------------------+
~~~

It seems we're good. The REMOTE_IP seems to be working as well, but since we have curl here,
let's forge some headers and verify that too? We need to verify XFF and XRI headers respectively:

- XFF with `-H "X-Forwarded-For: 8.8.8.8, 127.0.0.1"`
- XRI with `-X "X-Real-IP: 9.9.9.9"`

~~~plaintext
+--------------------+----------+------------------+-------------+-----------+---------------------+
| id                 | property | property_section | property_id | remote_ip | stamp               |
+--------------------+----------+------------------+-------------+-----------+---------------------+
| 279836335454289923 | news     |                1 |           1 | 9.9.9.9   | 2019-12-14 12:04:14 |
| 279836312486281219 | news     |                1 |           1 | 8.8.8.8   | 2019-12-14 12:04:00 |
+--------------------+----------+------------------+-------------+-----------+---------------------+
~~~

In the words of John "Hannibal" Smith: "*I love it when a plan comes together*".