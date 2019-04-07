### How to run operator on minikube

Install [minikube](https://github.com/kubernetes/minikube).

Execute `eval $(minikube docker-env)` to use Docker from minikube.

Build test service Docker image:

    docker build --file test_service/Dockerfile --tag test-service:latest .

Setup `MonitoredService` CRD:

    kubectl create -f deploy/crds/services_v1alpha1_monitoredservice_crd.yaml
    
Setup Service Account:

    kubectl create -f deploy/service_account.yaml
    
Setup RBAC:

    kubectl create -f deploy/role.yaml
    kubectl create -f deploy/role_binding.yaml
    
Enable ingress on minikube:

    minikube addons enable ingress

Create test service and ingress:

    kubectl create -f deploy/service.yaml
    kubectl create -f deploy/ingress.yaml
    
Update your `/etc/hosts` to include:

    $(minikube ip) test-service-local
    
Run operator and create `MonitoredService` resource for test service:

     operator-sdk up local --namespace=default
     kubectl create -f deploy/crds/services_v1alpha1_monitoredservice_cr.yaml
     
In short time test service should return responses:

    curl test-service-local
     