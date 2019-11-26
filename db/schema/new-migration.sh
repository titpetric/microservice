#!/bin/sh
if [ -z "$1" ]; then
	echo "Usage: ./$(basename $0) [service]"
	exit 255
fi

service=$1
if [ ! -d "$service" ]; then
	mkdir $service
fi

cp migrations.sql $service/
touch $service/$(date +%Y-%m-%d-%H%M%S)-description-here.up.sql
