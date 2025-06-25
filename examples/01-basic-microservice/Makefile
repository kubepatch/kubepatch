image:
	docker buildx build -t localhost:5000/restapiapp .
	docker buildx build -t localhost:5000/restapiapp:1.21 .
	docker buildx build -t localhost:5000/restapiapp:1.22 .
	docker push localhost:5000/restapiapp:1.21
	docker push localhost:5000/restapiapp:1.22
	docker push localhost:5000/restapiapp:latest

binary:
	@cd ../../ && make install > /dev/null 2>&1

dev-render:
	@kubepatch patch -f k8s/manifests -p k8s/patches-dev.yaml
prod-render:
	@kubepatch patch -f k8s/manifests -p k8s/patches-prod.yaml

dev-deploy:
	@kubepatch patch -f k8s/manifests -p k8s/patches-dev.yaml | kubectl apply -f -
prod-deploy:
	@kubepatch patch -f k8s/manifests -p k8s/patches-prod.yaml | kubectl apply -f -

dev-ping:
	@curl --location 'http://localhost:30265/status'
prod-ping:
	@curl --location 'http://localhost:30266/status'
