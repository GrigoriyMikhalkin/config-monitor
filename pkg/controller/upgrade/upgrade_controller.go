package upgrade

import (
	"context"
	servicesv1alpha1 "github.com/GrigoriyMikhalkin/config-monitor/pkg/apis/services/v1alpha1"
	"github.com/GrigoriyMikhalkin/config-monitor/pkg/controller/common"
	"k8s.io/apimachinery/pkg/labels"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_upgrade")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Upgrade Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, eventsChan chan event.GenericEvent) error {
	return add(mgr, newReconciler(mgr, eventsChan))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, eventsChan <-chan event.GenericEvent) reconcile.Reconciler {
	return &ReconcileUpgrade{client: mgr.GetClient(), scheme: mgr.GetScheme(), eventsChan: eventsChan}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("upgrade-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for replica size changes
	err = c.Watch(&source.Kind{Type: &servicesv1alpha1.MonitoredService{}}, &handler.EnqueueRequestForObject{}, common.ReplicaSizeUpdatePredicate)
	if err != nil {
		return err
	}

	// Watch for config updates
	err = c.Watch(&source.Channel{Source: r.(*ReconcileUpgrade).eventsChan}, &handler.EnqueueRequestsFromMapFunc{ToRequests: common.MapToMonitoredServiceObject})
	if err != nil {
		return nil
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileUpgrade{}

// ReconcileUpgrade reconciles a Upgrade object
type ReconcileUpgrade struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client 		client.Client
	scheme 		*runtime.Scheme
	eventsChan	<-chan event.GenericEvent
}

func (r *ReconcileUpgrade) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Upgrade")

	// Fetch the MonitoredService instance
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

	// Check if Deployment with service already exists, if not create a new one
	dep := &appsv1.Deployment{}
	err = r.client.Get(context.Background(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, dep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep = r.deploymentForService(instance)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.Background(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue request
		instance.Status.ConfigChanged = false
		err = r.client.Status().Update(context.Background(), instance)
		return reconcile.Result{Requeue: true}, nil
	}  else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := instance.Spec.Size
	if *dep.Spec.Replicas != size {
		dep.Spec.Replicas = &size
		err = r.client.Update(context.Background(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}

	if instance.Status.ConfigChanged {
		err = r.updatePodsSpec(dep, instance.Status.PodSpec)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		instance.Status.ConfigChanged = false
		err = r.client.Status().Update(context.Background(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update service status", "Service.Namespace", instance.Namespace, "Service.Name", instance.Name)
			return reconcile.Result{}, err
		}
		// Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}

	// Update the service status with the pod names
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForService(instance.Name))
	listOps := &client.ListOptions{Namespace: instance.Namespace, LabelSelector: labelSelector}
	err = r.client.List(context.Background(), listOps, podList)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods", "Service.Namespace", instance.Namespace, "Service.Name", instance.Name)
		return reconcile.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, instance.Status.Nodes) {
		instance.Status.Nodes = podNames
		err = r.client.Status().Update(context.Background(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update service status")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// returns the labels for selecting the resources belonging to service with given name
func labelsForService(name string) map[string]string {
	return map[string]string{"app": "monitored_service", "monitored_service_cr": name}
}

func (r *ReconcileUpgrade) deploymentForService(service *servicesv1alpha1.MonitoredService) *appsv1.Deployment {
	ls := labelsForService(service.Name)
	replicas := service.Spec.Size

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: service.Name,
			Namespace: service.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: service.Status.PodSpec,
			},
		},
	}

	// Set MonitoredService instance as owner of this deployment
	controllerutil.SetControllerReference(service, dep, r.scheme)
	return dep
}

func (r *ReconcileUpgrade) updatePodsSpec(dep *appsv1.Deployment, spec corev1.PodSpec) error {
	dep.Spec.Template.Spec = spec
	err := r.client.Update(context.Background(), dep)
	return err
}

func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
