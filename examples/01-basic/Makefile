image:
	@cd restapiapp && bash build.sh > /dev/null 2>&1
binary:
	@cd ../../ && make install > /dev/null 2>&1

dev-render:
	@kubepatch patch -f manifests -p patches-dev.yaml
prod-render:
	@kubepatch patch -f manifests -p patches-prod.yaml

dev-deploy:
	@kubepatch patch -f manifests -p patches-dev.yaml | kubectl apply -f -
prod-deploy:
	@kubepatch patch -f manifests -p patches-prod.yaml | kubectl apply -f -

dev-ping:
	@curl --location 'http://localhost:30265/status'
prod-ping:
	@curl --location 'http://localhost:30266/status'
