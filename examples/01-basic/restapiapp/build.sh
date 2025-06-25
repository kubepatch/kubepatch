#!/bin/bash
set -euo pipefail

docker buildx build -t localhost:5000/restapiapp .
docker buildx build -t localhost:5000/restapiapp:1.21 .
docker buildx build -t localhost:5000/restapiapp:1.22 .
docker push localhost:5000/restapiapp:1.21
docker push localhost:5000/restapiapp:1.22
docker push localhost:5000/restapiapp:latest
