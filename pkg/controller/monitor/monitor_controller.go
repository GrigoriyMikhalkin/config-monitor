package monitor

import (
	"bufio"
	"context"
	servicesv1alpha1 "github.com/GrigoriyMikhalkin/config-monitor/pkg/apis/services/v1alpha1"
	"github.com/GrigoriyMikhalkin/config-monitor/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
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
	err = c.Watch(&source.Kind{Type: &servicesv1alpha1.MonitoredService{}}, &handler.EnqueueRequestForObject{}, common.CreateOrUpdateSpecPredicate)
	if err != nil {
		return err
	}

	// Watch for service config updates
	monitorChan := make(chan event.GenericEvent)

	mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		e := event.GenericEvent{Meta: &servicesv1alpha1.MonitoredService{}}
		monitorTicker := time.NewTicker(time.Second * 20)

		for _ = range monitorTicker.C {
			monitorChan <- e
		}
		return nil
	}))
	err = c.Watch(&source.Channel{Source: monitorChan}, &handler.EnqueueRequestsFromMapFunc{ToRequests: common.GetToMonitorServicesMapper(mgr)})
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

func (r *ReconcileMonitor) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Checking updates in repo for MonitoredService")

	// fetch the Monitor instance
	instance := &servicesv1alpha1.MonitoredService{}
	err := r.client.Get(context.Background(), request.NamespacedName, instance)
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

	// check if service's config was updated
	// if it was, send event to upgrade controller
	if podSpec, ok := r.isServiceConfigUpdated(instance); ok {
		// Update instance Spec
		instance.Status.PodSpec = *podSpec
		instance.Status.ConfigChanged = true
		err = r.client.Status().Update(context.Background(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update service status", "Service.Namespace", instance.Namespace, "Service.Name", instance.Name)
			return reconcile.Result{}, err
		}
		r.eventsChan <- event.GenericEvent{Meta: &servicesv1alpha1.MonitoredService{}, Object: instance}
	}

	return reconcile.Result{}, nil
}

// Check if service's config is updated
func (r *ReconcileMonitor) isServiceConfigUpdated(service *servicesv1alpha1.MonitoredService) (*corev1.PodSpec, bool) {
	updated := false
	podSpec := service.Status.PodSpec

	// if service's podSpec is empty, create new one
	if len(podSpec.Containers) < 1 {
		podSpec = corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: service.Name,
					Image: service.Spec.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: *service.Spec.Port,
							Name: "service",
						},
					},
				},
			},
		}
	}

	// fetch config
	resp, err := http.Get(service.Spec.ConfigRepo)
	defer resp.Body.Close()
	if err != nil {
		return nil, updated
	}

	envVars := []corev1.EnvVar{}
	scanner := bufio.NewScanner(resp.Body)
	for {
		next := scanner.Scan()
		if !next {
			break
		}

		line := strings.SplitN(string(scanner.Bytes()), "=", 2)
		if len(line) == 2 {
			envVars = append(envVars, corev1.EnvVar{Name: line[0], Value: line[1]})
		}
	}

	// compare current env vars and new
	if !reflect.DeepEqual(envVars, podSpec.Containers[0].Env) {
		updated = true
		podSpec.Containers[0].Env = envVars
	}

	return &podSpec, updated
}