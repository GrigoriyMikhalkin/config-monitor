package common

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	servicesv1alpha1 "github.com/GrigoriyMikhalkin/config-monitor/pkg/apis/services/v1alpha1"
)

var CreateOrConfigRepoUpdatePredicate = predicate.Funcs{
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldService := e.ObjectOld.(*servicesv1alpha1.MonitoredService)
		newService := e.ObjectNew.(*servicesv1alpha1.MonitoredService)

		return oldService.Spec.ConfigRepo != newService.Spec.ConfigRepo
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

var ReplicaSizeUpdatePredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldService := e.ObjectOld.(*servicesv1alpha1.MonitoredService)
		newService := e.ObjectNew.(*servicesv1alpha1.MonitoredService)

		return oldService.Spec.Size != newService.Spec.Size
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}