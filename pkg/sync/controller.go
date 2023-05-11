package sync

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/philipgough/hashring-controller/pkg/controller"
	"github.com/prometheus/client_golang/prometheus"

	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
)

const (
	defaultPath = "/var/lib/thanos-receive/hashring.json"
	// configMapKey is the default key for the generated ConfigMap
	configMapKey = controller.DefaultConfigMapKey

	// defaultSyncBackOff is the default backoff period for syncService calls.
	defaultSyncBackOff = 1 * time.Second
	// maxSyncBackOff is the max backoff period for sync calls.
	maxSyncBackOff = 1000 * time.Second
)

type Controller struct {
	// client is the kubernetes client
	client clientset.Interface

	// configMapLister is able to list/get configmaps and is populated by the
	// shared informer passed to NewController
	configMapLister corelisters.ConfigMapLister
	// configMapSynced returns true if the configmaps shared informer has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	configMapSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	// workerLoopPeriod is the time between worker runs. The workers
	// process the queue of service and pod changes
	workerLoopPeriod time.Duration

	// configMapName is the name of the configmap that the controller will generate
	configMapName string
	// namespace is the namespace of the configmap that the controller will generate
	namespace string

	logger  log.Logger
	metrics *metrics

	path string
	key  string
}

type Options struct {
	// ConfigMapKey is the key for hashring config on the generated ConfigMap
	ConfigMapKey *string
	// ConfigMapName is the name of the generated ConfigMap
	ConfigMapName *string
	// FilePath is the path to the hashring.json file
	FilePath *string
}

func NewController(
	ctx context.Context,
	configMapInformer coreinformers.ConfigMapInformer,
	client clientset.Interface,
	namespace string,
	logger log.Logger,
	registry *prometheus.Registry,
	opts *Options,
) *Controller {

	if logger == nil {
		logger = log.NewNopLogger()
	}

	ctrlMetrics := newMetrics()
	if registry == nil {
		registry.MustRegister(ctrlMetrics.configMapHash, ctrlMetrics.configMapLastWriteSuccessTime)
	}

	opts = buildOpts(opts)

	c := &Controller{
		client:        client,
		configMapName: *opts.ConfigMapName,
		// This is similar to the DefaultControllerRateLimiter, just with a
		// significantly higher default backoff (1s vs 5ms). A more significant
		// rate limit back off here helps ensure that the Controller does not
		// overwhelm the API Server.
		queue: workqueue.NewRateLimitingQueueWithConfig(
			workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(defaultSyncBackOff, maxSyncBackOff),
				// 10 qps, 100 bucket size.
				// This is only for retry speed and is only the overall factor (not per item).
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			workqueue.RateLimitingQueueConfig{Name: "configmap_sync"}),
		workerLoopPeriod: time.Second,
		namespace:        namespace,
		logger:           logger,
		metrics:          ctrlMetrics,
		path:             *opts.FilePath,
		key:              *opts.ConfigMapKey,
	}

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onConfigMapAdd,
		UpdateFunc: c.onConfigMapUpdate,
		DeleteFunc: c.onConfigMapDelete,
	})

	c.configMapLister = configMapInformer.Lister()
	c.configMapSynced = configMapInformer.Informer().HasSynced

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the queue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	level.Info(c.logger).Log("msg", "starting hashring sync controller")
	level.Info(c.logger).Log("msg", "waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.configMapSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	level.Info(c.logger).Log("msg", "starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	level.Info(c.logger).Log("msg", "started workers")
	<-ctx.Done()
	level.Info(c.logger).Log("msg", "shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the queue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// enqueueConfigMap takes a ConfigMap resource
// It converts it into a namespace/name string which is then put onto the queue.
// This method should *not* be passed resources of any type other than ConfigMap.
func (c *Controller) enqueueConfigMap(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// processNextWorkItem will read a single work item off the queue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.queue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item being re-queued.
		// For example, we do not call Forget if a transient error occurs, instead the item is
		// put back on the queue and attempted again after a back-off period.
		defer c.queue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the queue which are of the form namespace/name.
		// We do this as the delayed nature of the queue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the queue.
		if key, ok = obj.(string); !ok {
			// As the item in the queue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the ConfigMap resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the queue to handle any transient errors.
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		level.Debug(c.logger).Log("msg", "successfully synced", "resourceName", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to converge the two.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	level.Debug(c.logger).Log("msg", "syncHandler called", "resourceName", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the ConfigMap resource with this namespace/name
	cm, err := c.configMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		// The ConfigMap resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ConfigMap '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	data, ok := cm.Data[c.key]
	if !ok {
		level.Warn(c.logger).Log("msg", "no data found for key", "key", c.key)
		return nil
	}

	if err := os.WriteFile(c.path, []byte(data), 0644); err != nil {
		level.Error(c.logger).Log("err", fmt.Sprintf("failed to write file: %v", err))
		return err
	}
	c.metrics.configMapLastWriteSuccessTime.Set(float64(time.Now().Unix()))
	c.metrics.configMapHash.Set(hashAsMetricValue([]byte(data)))
	return nil
}

func (c *Controller) onConfigMapAdd(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		level.Error(c.logger).Log("msg", "unexpected object type", "type", fmt.Sprintf("%T", obj))
		return
	}

	if !c.shouldEnqueue(cm) {
		return
	}
	c.enqueueConfigMap(cm)
}

func (c *Controller) onConfigMapUpdate(oldObj, newObj interface{}) {
	newCM := newObj.(*corev1.ConfigMap)
	oldCM := oldObj.(*corev1.ConfigMap)

	if !c.shouldEnqueue(newCM) {
		return
	}

	if newCM.ResourceVersion == oldCM.ResourceVersion {
		// Periodic resync will send update events for all known ConfigMap.
		// Two different versions will always have different RVs.
		return
	}

	c.enqueueConfigMap(newCM)
}

func (c *Controller) onConfigMapDelete(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		level.Error(c.logger).Log("msg", "unexpected object type", "type", fmt.Sprintf("%T", obj))
		return
	}
	level.Info(c.logger).Log("msg", "ConfigMap deleted - ignoring", "name", cm.Name, "namespace", cm.Namespace)
}

func (c *Controller) shouldEnqueue(cm *corev1.ConfigMap) bool {
	if value, ok := cm.GetLabels()[controller.ConfigMapLabel]; !ok || value != "true" {
		return false
	}
	return true
}

func buildOpts(o *Options) *Options {
	if o == nil {
		o = &Options{
			ConfigMapName: pointer.String(controller.DefaultConfigMapName),
			ConfigMapKey:  pointer.String(controller.DefaultConfigMapKey),
			FilePath:      pointer.String(defaultPath),
		}
	}
	if o.ConfigMapName == nil {
		o.ConfigMapName = pointer.String(controller.DefaultConfigMapName)
	}
	if o.ConfigMapKey == nil {
		o.ConfigMapKey = pointer.String(controller.DefaultConfigMapKey)
	}
	if o.FilePath == nil {
		o.FilePath = pointer.String(defaultPath)
	}
	return o
}
