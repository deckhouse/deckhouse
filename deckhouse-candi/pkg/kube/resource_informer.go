package kube

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infappsv1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

type DeploymentInformer struct {
	KubeClient *KubernetesClient
	ctx        context.Context
	cancel     context.CancelFunc

	// Filter by namespace
	Namespace string
	// Filter by object name
	Name string
	// filter labels
	LabelSelector *metav1.LabelSelector
	// filter by fields
	FieldSelector string

	SharedInformer cache.SharedInformer

	ListOptions metav1.ListOptions

	EventCb func(obj *appsv1.Deployment, event string)
}

func NewDeploymentInformer(client *KubernetesClient, parentCtx context.Context) *DeploymentInformer {
	ctx, cancel := context.WithCancel(parentCtx)
	informer := &DeploymentInformer{
		KubeClient: client,
		ctx:        ctx,
		cancel:     cancel,
	}
	return informer
}

func (p *DeploymentInformer) WithKubeEventCb(eventCb func(obj *appsv1.Deployment, event string)) {
	p.EventCb = eventCb
}

func (p *DeploymentInformer) CreateSharedInformer() (err error) {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// define tweakListOptions for informer
	labelSelector, err := metav1.LabelSelectorAsSelector(p.LabelSelector)
	if err != nil {
		return err
	}

	tweakListOptions := func(options *metav1.ListOptions) {
		if p.FieldSelector != "" {
			options.FieldSelector = p.FieldSelector
		}
		if labelSelector.String() != "" {
			options.LabelSelector = labelSelector.String()
		}
	}
	//p.ListOptions = metav1.ListOptions{}
	//tweakListOptions(&p.ListOptions)

	// create informer with add, update, delete callbacks
	informer := infappsv1.NewFilteredDeploymentInformer(p.KubeClient, p.Namespace, resyncPeriod, indexers, tweakListOptions)
	informer.AddEventHandler(p)
	p.SharedInformer = informer

	return nil
}

//// ListExistedObjects get a list of existed objects in namespace that match selectors and
//// fills Checksum map with checksums of existing objects.
//func (ei *DeploymentInformer) ListExistedObjects() error {
//	objList, err := ei.KubeClient.Dynamic().
//		Resource(ei.GroupVersionResource).
//		Namespace(ei.Namespace).
//		List(ei.ListOptions)
//	if err != nil {
//		log.Errorf("%s: initial list resources of kind '%s': %v", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, err)
//		return err
//	}
//
//	if objList == nil || len(objList.Items) == 0 {
//		log.Debugf("%s: Got no existing '%s' resources", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind)
//		return nil
//	}
//
//	// FIXME objList.Items has too much information for log
//	//log.Debugf("%s: Got %d existing '%s' resources: %+v", ei.Monitor.Metadata.DebugName, len(objList.Items), ei.Monitor.Kind, objList.Items)
//	log.Debugf("%s: '%s' initial list: Got %d existing resources", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, len(objList.Items))
//
//	var filteredObjects = make(map[string]*ObjectAndFilterResult)
//
//	for _, item := range objList.Items {
//		// copy loop var to avoid duplication of pointer
//		obj := item
//		objFilterRes, err := ApplyJqFilter(ei.Monitor.JqFilter, &obj)
//		if err != nil {
//			return err
//		}
//		// save object to the cache
//
//		filteredObjects[objFilterRes.Metadata.ResourceId] = objFilterRes
//
//		log.Debugf("%s: initial list: '%s' is cached with checksum %s",
//			ei.Monitor.Metadata.DebugName,
//			objFilterRes.Metadata.ResourceId,
//			objFilterRes.Metadata.Checksum)
//	}
//
//	ei.cacheLock.Lock()
//	defer ei.cacheLock.Unlock()
//	for k, v := range filteredObjects {
//		ei.CachedObjects[k] = v
//	}
//
//	return nil
//}

func (p *DeploymentInformer) OnAdd(obj interface{}) {
	p.HandleWatchEvent(obj, "Added")
}

func (p *DeploymentInformer) OnUpdate(oldObj, newObj interface{}) {
	p.HandleWatchEvent(newObj, "Modified")
}

func (p *DeploymentInformer) OnDelete(obj interface{}) {
	p.HandleWatchEvent(obj, "Deleted")
}

// HandleKubeEvent register object in cache. Pass object to callback if object's checksum is changed.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modified is enough)
//func (ei *DeploymentInformer) HandleKubeEvent(obj *unstructured.Unstructured, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
func (p *DeploymentInformer) HandleWatchEvent(object interface{}, eventType string) {
	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	var obj = object.(*appsv1.Deployment)

	p.EventCb(obj, eventType)
	//
	//// Ignore Added or Modified if object is in cache and its checksum is equal to the newChecksum.
	//// Delete is never ignored.
	//switch eventType {
	//case "Added":
	//	fallthrough
	//case "Modified":
	//	// Update object in cache
	//	ei.cacheLock.Lock()
	//	cachedObject, objectInCache := ei.CachedObjects[resourceId]
	//	skipEvent := false
	//	if objectInCache && cachedObject.Metadata.Checksum == objFilterRes.Metadata.Checksum {
	//		// update object in cache and do not send event
	//		log.Debugf("%s: %s %s: checksum is not changed, no KubeEvent",
	//			ei.Monitor.Metadata.DebugName,
	//			string(eventType),
	//			resourceId,
	//		)
	//		skipEvent = true
	//	}
	//	ei.CachedObjects[resourceId] = objFilterRes
	//	ei.cacheLock.Unlock()
	//	if skipEvent {
	//		return
	//	}
	//
	//case "Deleted":
	//	ei.cacheLock.Lock()
	//	delete(ei.CachedObjects, resourceId)
	//	ei.cacheLock.Unlock()
	//}
	//
	//// Fire KubeEvent only if needed.
	//if ei.ShouldFireEvent(eventType) {
	//	log.Debugf("%s: %s %s: send KubeEvent",
	//		ei.Monitor.Metadata.DebugName,
	//		string(eventType),
	//		resourceId,
	//	)
	//	// TODO: should be disabled by default and enabled by a debug feature switch
	//	//log.Debugf("HandleKubeEvent: obj type is %T, value:\n%#v", obj, obj)
	//
	//	// Pass event info to callback
	//	ei.EventCb(KubeEvent{
	//		MonitorId:   ei.Monitor.Metadata.MonitorId,
	//		WatchEvents: []WatchEventType{eventType},
	//		Objects:     []ObjectAndFilterResult{*objFilterRes},
	//	})
	//}
}

//func (ei *DeploymentInformer) adjustFieldSelector(selector *FieldSelector, objName string) *FieldSelector {
//	var selectorCopy *FieldSelector
//
//	if selector != nil {
//		selectorCopy = &FieldSelector{
//			MatchExpressions: selector.MatchExpressions,
//		}
//	}
//
//	if objName != "" {
//		objNameReq := FieldSelectorRequirement{
//			Field:    "metadata.name",
//			Operator: "=",
//			Value:    objName,
//		}
//		if selectorCopy == nil {
//			selectorCopy = &FieldSelector{
//				MatchExpressions: []FieldSelectorRequirement{
//					objNameReq,
//				},
//			}
//		} else {
//			selectorCopy.MatchExpressions = append(selectorCopy.MatchExpressions, objNameReq)
//		}
//	}
//
//	return selectorCopy
//}

func (p *DeploymentInformer) Run() {
	p.SharedInformer.Run(p.ctx.Done())
}

func (p *DeploymentInformer) Stop() {
	p.cancel()
}
