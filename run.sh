#!/bin/bash

cd /root/sensors_api
docker rm -f sensors_api
docker image rm ghcr.io/pgulb/sensors_api:main
docker pull ghcr.io/pgulb/sensors_api:main
docker run -d --restart unless-stopped --name sensors_api -p 3333:3000 -v ./db.json:/app/db.json:ro ghcr.io/pgulb/sensors_api:main
