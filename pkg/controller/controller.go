package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, chan event.GenericEvent) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	eventsChan := make(chan event.GenericEvent)

	for _, f := range AddToManagerFuncs {
		if err := f(m, eventsChan); err != nil {
			return err
		}
	}
	return nil
}
