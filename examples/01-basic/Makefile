image:
	@cd restapiapp && bash build.sh > /dev/null 2>&1

binary:
	@cd ../../ && make install > /dev/null 2>&1

dev:
	@kubepatch patch -f manifests -p patches-dev.yaml

prod:
	@kubepatch patch -f manifests -p patches-prod.yaml

dev-deploy:
	@kubepatch patch -f manifests -p patches-dev.yaml | kubectl apply -f -

prod-deploy:
	@kubepatch patch -f manifests -p patches-prod.yaml | kubectl apply -f -

ping-dev:
	@curl --location 'http://localhost:30265/status'

ping-prod:
	@curl --location 'http://localhost:30266/status'
