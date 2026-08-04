# webapp-operator-kb

An Kubernetes Operator for deploying web applications. The operator manages deployments, SVCs, Traefik CRDS + Ingress Controller, IngressRoutes(Traefik), and more in the future. <br>
**NOTE: current implementation does not create the deployment for the operator so you have to do it manually by starting the Go application/**

## Requirements
* Go
* Make
* A working cluster

## Local dev setup (using minikube)
* Install CRDs into the cluster using ```make install```.
* Start the controller using ```go run ./cmd```
* Create a web application manifest. Use config/samples/operator_v1_webapp.yaml as reference.