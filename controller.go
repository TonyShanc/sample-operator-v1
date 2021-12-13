package main

import (
	"context"
	"fmt"

	"time"

	"github.com/golang/glog"
	samplecrdv1 "github.com/tonyshanc/sample-operator-v1/pkg/apis/samplecrd/v1"
	clientset "github.com/tonyshanc/sample-operator-v1/pkg/client/clientset/versioned"
	carscheme "github.com/tonyshanc/sample-operator-v1/pkg/client/clientset/versioned/scheme"
	informers "github.com/tonyshanc/sample-operator-v1/pkg/client/informers/externalversions/samplecrd/v1"
	listers "github.com/tonyshanc/sample-operator-v1/pkg/client/listers/samplecrd/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const controllerAgentName = "car-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Car is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Car
	// is synced successfully
	MessageResourced = "Car synced successfully"
)

// Controller is the controller implementation for Car resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// carclientset is a clientset for our own API group
	carclientset clientset.Interface

	carsLister listers.CarLister
	carsSynced cache.InformerSynced
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func NewController(
	kubeclientset kubernetes.Interface,
	carclientset clientset.Interface,
	carInformer informers.CarInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(carscheme.AddToScheme(scheme.Scheme))
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset: kubeclientset,
		carclientset:  carclientset,
		carsLister:    carInformer.Lister(),
		carsSynced:    carInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cars"),
		recorder:      recorder,
	}

	glog.Info("Setting up event handlers")

	// Set up event handler for when Car resources change
	carInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCar,
		UpdateFunc: func(old, new interface{}) {
			oldCar := old.(*samplecrdv1.Car)
			newCar := new.(*samplecrdv1.Car)
			if oldCar.ResourceVersion == newCar.ResourceVersion {
				// Periodic resync will send update events for all known Cars.
				// Two different versions of the same Car will always have different RVs.
				return
			}
			controller.enqueueCar(new)
		},
		DeleteFunc: controller.enqueueCarForDelete,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workerNum int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting car control loop")
	// Waiting for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.carsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Network resources
	for i := 0; i < workerNum; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Starting workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Car resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Car resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	car, err := c.carsLister.Cars(namespace).Get(name)
	if err != nil {
		// The Car resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			glog.Warningf("Car: %s/%s does not exist in local cache, will delete it from carset",
				namespace, name)

			glog.Infof("Deleting Car: %s/%s ...", namespace, name)

			return nil
		}
		runtime.HandleError(fmt.Errorf("failed to list car by %s/%s", namespace, name))
		return err
	}

	c.carclientset.SamplecrdV1().Cars(car.Namespace).Create(context.TODO(), car, v1.CreateOptions{})
	c.recorder.Event(car, corev1.EventTypeNormal, SuccessSynced, MessageResourced)
	return nil
}

// enqueueCar takes a Car resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should not be
// passed resources of any type other than Car.
func (c *Controller) enqueueCar(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// enqueueCarForDelete takes a deleted Car resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should not be
// passed resources of any type other than Car.
func (c *Controller) enqueueCarForDelete(obj interface{}) {
	var key string
	var err error
	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}
