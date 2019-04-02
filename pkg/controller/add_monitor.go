package controller

import (
	"github.com/GrigoriyMikhalkin/config-monitor/pkg/controller/monitor"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, monitor.Add)
}
