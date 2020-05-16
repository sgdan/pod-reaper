.PHONY: build go

go:
	docker build -t gotest .
	docker run --rm -it -v $(HOME)/.kube:/root/.kube -p 8080:8080 gotest

# assumes go installed locally
test:
	go mod tidy
	go mod download
	go test -v ./cmd/reaper


# build and run using docker
build:
	docker build . -t podreaper

# run: build
# 	docker run --rm -it -v ~/.kube:/root/.kube -p 8080:8080 podreaper

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
frontend-dev:
	cd frontend && elm-app start

backend-dev:
	docker-compose run -p 8080:8080 gradle gradle run


# unit testing
frontend-test:
	cd frontend && elm-test

backend-test:
	docker-compose run --rm gradle gradle test


# gradle shell for back end
backend-shell:
	docker-compose run --rm -p 8080:8080 gradle bash


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
	kubectl edit cm podreaper-config -n podreaper
