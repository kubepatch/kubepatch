dev:
	@go run "../../main.go" manifests patches-dev.yaml

prod:
	@go run "../../main.go" manifests patches-prod.yaml

ping-dev:
	@curl --location 'http://localhost:30265/status'

ping-prod:
	@curl --location 'http://localhost:30266/status'

