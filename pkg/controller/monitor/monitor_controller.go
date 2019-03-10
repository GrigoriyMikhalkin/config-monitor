package monitor

import (
	"context"
	"encoding/json"
	servicesv1alpha1 "github.com/GrigoriyMikhalkin/git-monitor/pkg/apis/services/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

var log = logf.Log.WithName("controller_monitor")

// Add creates a new Monitor Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, eventsChan chan event.GenericEvent) error {
	return add(mgr, newReconciler(mgr, eventsChan))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, eventsChan chan<- event.GenericEvent) reconcile.Reconciler {
	return &ReconcileMonitor{client: mgr.GetClient(), scheme: mgr.GetScheme(), eventsChan: eventsChan}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("monitor-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for new MonitoredServices and for repository link updates
	p := predicate.Funcs{
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
	err = c.Watch(&source.Kind{Type: &servicesv1alpha1.MonitoredService{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	// Watch for service config updates
	var mapper handler.ToRequestsFunc = func(object handler.MapObject) []reconcile.Request {
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
	monitorChan := make(chan event.GenericEvent)

	mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		e := event.GenericEvent{Meta: &servicesv1alpha1.MonitoredService{}}
		monitorTicker := time.NewTicker(time.Second * 5)

		for _ = range monitorTicker.C {
			monitorChan <- e
		}
		return nil
	}))
	err = c.Watch(&source.Channel{Source: monitorChan}, &handler.EnqueueRequestsFromMapFunc{ToRequests: mapper})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMonitor{}

// ReconcileMonitor reconciles a Monitor object
type ReconcileMonitor struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client 		client.Client
	scheme 		*runtime.Scheme
	eventsChan	chan<- event.GenericEvent
}

// Reconcile reads that state of the cluster for a Monitor object and makes changes based on the state read
// and what is in the Monitor.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMonitor) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Checking updates in Git for MonitoredService")

	// Fetch the Monitor instance
	instance := &servicesv1alpha1.MonitoredService{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Check updates for service
	currentConfig := corev1.PodSpec{}
	instanceConfig := instance.Status.PodSpec

	// Fetch config
	resp, err := http.Get(instance.Spec.ConfigRepo)
	defer resp.Body.Close()
	if err != nil {
		reqLogger.Error(err, "Failed to fetch service config", "Service.Namespace", instance.Namespace, "Service.Name", instance.Name)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, currentConfig)
	if err != nil {
		reqLogger.Error(err, "Failed to parse service config, must be invalid JSON", "Service.Namespace", instance.Namespace, "Service.Name", instance.Name)
	}

	// Compare current config and instance config
	if !reflect.DeepEqual(currentConfig, instanceConfig) {
		// Update instance Spec
		instance.Status.PodSpec = currentConfig
		instance.Status.SpecChanged = true
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update service status", "Service.Namespace", instance.Namespace, "Service.Name", instance.Name)
			return reconcile.Result{}, err
		}
		r.eventsChan <- event.GenericEvent{Meta: &servicesv1alpha1.MonitoredService{}, Object: instance}
	}

	return reconcile.Result{}, nil
}
