#!/bin/bash

cd /root/sensors_api
docker rm -f sensors_api
docker image rm ghcr.io/pgulb/sensors_api:main
docker pull ghcr.io/pgulb/sensors_api:main
docker run -d --name sensors_api -v ./db.json:/app/db.json:ro ghcr.io/pgulb/sensors_api:main
