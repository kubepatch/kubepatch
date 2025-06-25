# Quick Start

```
# build docker image for a simple web-app, push to a local registry
make image

# render manifests with applying 'dev' patch-set (just debug to stdout)
make dev

# render manifests with applying 'prod' patch-set (just debug to stdout)
make prod

# deploy patched manifests for 'dev' environment
make dev-deploy

# deploy patched manifests for 'prod' environment
make prod

# just curl running apps
make ping-dev
make ping-prod
```
