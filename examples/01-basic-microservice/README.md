# Quick Start

```
######################################################################
### prepare image/binary
######################################################################

# build docker image for a simple web-app, push to a local registry
make image

# build and install kubepatch to /usr/local/bin
make binary

######################################################################
### test steps
######################################################################

# render manifests with applying 'dev' patch-set (just debug to stdout)
make dev-render

# render manifests with applying 'prod' patch-set (just debug to stdout)
make prod-render

# deploy patched manifests for 'dev' environment
make dev-deploy

# deploy patched manifests for 'prod' environment
make prod-deploy

# just curl running apps
make dev-ping
make prod-ping
```
