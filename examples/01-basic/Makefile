image:
	@cd restapiapp && bash build.sh

dev:
	@go run "../../main.go" patch -f manifests -p patches-dev.yaml

prod:
	@go run "../../main.go" patch -f manifests -p patches-prod.yaml

dev-deploy: image
	@go run "../../main.go" patch -f manifests -p patches-dev.yaml | kubectl apply -f -

prod-deploy: image
	@go run "../../main.go" patch -f manifests -p patches-prod.yaml | kubectl apply -f -

ping-dev:
	@curl --location 'http://localhost:30265/status'

ping-prod:
	@curl --location 'http://localhost:30266/status'

