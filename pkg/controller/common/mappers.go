package common

import (
	"context"
	servicesv1alpha1 "github.com/GrigoriyMikhalkin/config-monitor/pkg/apis/services/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func GetToMonitorServicesMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		mgrClient := mgr.GetClient()
		namespace, _ := k8sutil.GetWatchNamespace()
		servicesList := servicesv1alpha1.MonitoredServiceList{}
		_ = mgrClient.List(context.TODO(), &client.ListOptions{Namespace: namespace}, &servicesList)

		requests := make([]reconcile.Request, len(servicesList.Items))
		for ind, service := range servicesList.Items {
			namespaceName := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}
			req := reconcile.Request{namespaceName}
			requests[ind] = req
		}
		return requests
	}
}

var  MapToMonitoredServiceObject handler.ToRequestsFunc = func(object handler.MapObject) []reconcile.Request {
	service := object.Object.(*servicesv1alpha1.MonitoredService)
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: service.Name, Namespace: service.Namespace}},
	}
}