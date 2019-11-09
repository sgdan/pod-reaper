.PHONY: build

# build and run using docker
build:
	docker build . -t podreaper

run: build
	docker run --rm -it -v ~/.kube:/root/.kube -p 8080:8080 podreaper

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
	kubectl create deployment test1 --image=nginx -n ns1
	kubectl create deployment test2 --image=nginx -n ns2
	kubectl create deployment test3 --image=nginx -n ns3
	kubectl scale deployment test1 --replicas 8 -n ns1
	kubectl scale deployment test2 --replicas 10 -n ns2
	kubectl scale deployment test3 --replicas 12 -n ns3

reset:
	kubectl delete ns ns1 ns2 ns3

# edit to manually change last started time for test purposes
edit:
	kubectl edit cm podreaper-config -n podreaper
