#!/bin/bash

export SSH_PRV_KEY="$(cat ~/.ssh/id_rsa)"
export SSH_PUB_KEY="$(cat ~/.ssh/id_rsa.pub)"

docker-compose up -d --build
docker exec -it gotest /bin/sh