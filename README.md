### How to run operator on minikube

Install [minikube](https://github.com/kubernetes/minikube).

Build test service Docker image:

    docker build --file test_service/Dockerfile --tag test-service:latest .

Setup `MonitoredService` CRD:

    kubectl create -f deploy/crds/services_v1alpha1_monitoredservice_crd.yaml
    
Setup Service Account:

    kubectl create -f deploy/service_account.yaml
    
Setup RBAC:

    kubectl create -f deploy/role.yaml
    kubectl create -f deploy/role_binding.yaml
    
