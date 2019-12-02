#!/bin/bash
cd $(dirname $(dirname $(readlink -f $0)))

## encode file contents in base64
function base64_encode {
	cat $1 | base64 -w 0
}

## generate a service FS
function render_service_schema {
	local schema=$(basename $1)
	echo "package db"
	echo
	echo "var $schema FS = FS{"
	local files=$(find $1 -name '*.sql' | sort)
	for file in $files; do
		echo "\"$(basename $file)\": \"$(base64_encode $file)\","
	done
	echo "}"
}

## list all service FS into `migrations` global
function render_schema {
	echo "package db"
	echo
	echo "var migrations map[string]FS = map[string]FS{"
	for schema in $schemas; do
		local package=$(basename $schema)
		echo "\"${package}\": ${package},"
	done
	echo "}"
}

## list all service migrations (db/schema/stats, ...)
schemas=$(ls db/schema/*/migrations.sql | xargs -n1 dirname)
for schema in $schemas; do
	# db/schema/stats -> schema/stats
	schema_relative=${schema/db\//}
	# schema/stats -> db/schema_stats.go
	output="db/${schema_relative/\//_}.go"

	render_service_schema $schema > $output
	echo "~ $output"
done

render_schema > db/schema.go
echo "~ db/schema.go"
