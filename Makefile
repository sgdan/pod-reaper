.PHONY: build

# build and run using docker
build:
	docker build . -t podreaper

run: build
	docker run --rm -it \
		-v $(HOME)/.kube:/root/.kube -p 8080:8080 podreaper

# deploy to local kubernetes (make sure "podreaper" namespace exists)
# view UI at http://localhost:8080
deploy:
	kubectl apply -f deploy.yaml -n podreaper

forward:
	kubectl port-forward deployment/podreaper 8080:8080 -n podreaper

undeploy:
	kubectl delete -f deploy.yaml -n podreaper

logs:
	kubectl logs -f deployment/podreaper -n podreaper

events:
	kubectl get events --sort-by=.metadata.creationTimestamp --all-namespaces

# for local development, start front and back end separately
# Front end will run at http://localhost:3000
frontend-dev:
	cd frontend && npx parcel --port 3000 src/index.html

# Back end runs on localhost:8080, need CORS so dev front end can connect
# Assumes you have golang tools installed
backend-dev:
	go mod tidy
	go mod download
	go build -race -o reaper ./cmd/reaper
	CORS_ENABLED=true ./reaper


# unit testing
frontend-test:
	cd frontend && npx elm-test

# assumes golang is installed locally
# need vet=off workaround for windows, https://github.com/golang/go/issues/27089
backend-test:
	go mod tidy
	go mod download
	go test -race -vet=off -v ./cmd/reaper


# gradle shell for back end
backend-shell: build
	docker run --rm -it -e CORS_ENABLED=true \
		-v $(HOME)/.kube:/root/.kube -p 8080:8080 podreaper sh


# some resources for k8s testing
create:
	kubectl create ns ns1
	kubectl create ns ns2
	kubectl create ns ns3
	kubectl apply -f nginx.yaml -n ns1
	kubectl apply -f nginx.yaml -n ns2
	kubectl apply -f nginx.yaml -n ns3

scale-up:
	kubectl scale deployment test --replicas 4 -n ns1
	kubectl scale deployment test --replicas 8 -n ns2
	kubectl scale deployment test --replicas 12 -n ns3

scale-down:
	kubectl scale deployment test --replicas 0 -n ns1
	kubectl scale deployment test --replicas 0 -n ns2
	kubectl scale deployment test --replicas 0 -n ns3

delete:
	kubectl delete ns ns1 ns2 ns3

# edit to manually change last started time for test purposes
edit:
	kubectl edit cm podreaper-goconfig -n podreaper
