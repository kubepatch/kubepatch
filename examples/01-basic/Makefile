image:
	@cd restapiapp && bash build.sh

dev: image
	@go run "../../main.go" manifests patches-dev.yaml

prod: image
	@go run "../../main.go" manifests patches-prod.yaml

ping-dev:
	@curl --location 'http://localhost:30265/status'

ping-prod:
	@curl --location 'http://localhost:30266/status'

