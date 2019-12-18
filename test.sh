#!/bin/bash
function get_stats_ip {
	docker inspect \
		microservice_stats_1 | \
	jq -r '.[0].NetworkSettings.Networks["microservice_default"]'.IPAddress
}

url="http://$(get_stats_ip):3000/twirp/stats.StatsService/Push"

docker run --rm --net=host -it -v $PWD:/data skandyla/wrk -d60s -t4 -c100 -s test.lua $url

exit 0

payload='{
  "property": "news",
  "section": 1,
  "id": 1
}'

curl -s -X POST -H 'Content-Type: application/json' \
     -H "X-Real-IP: 9.9.9.9" \
     http://172.22.0.5:3000/twirp/stats.StatsService/Push \
     -d "$payload" | jq .
