package clusterlogforwardertemplate

import (
	"context"

	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &enqueueRequestForHostedControlPlane{}

type enqueueRequestForHostedControlPlane struct {
	Client client.Client
}

func (e *enqueueRequestForHostedControlPlane) mapAndEnqueue(q workqueue.RateLimitingInterface, obj client.Object, reqs map[reconcile.Request]struct{}) {
	for _, req := range e.mapToRequests(obj) {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			// Used for de-duping requests
			reqs[req] = struct{}{}
		}
	}
}

func (e *enqueueRequestForHostedControlPlane) mapToRequests(obj client.Object) []reconcile.Request {
	reqs := []reconcile.Request{}
	hcpList := &hyperv1beta1.HostedControlPlaneList{}

	err := e.Client.List(context.TODO(), hcpList, &client.ListOptions{Namespace: obj.GetNamespace()})
	if err != nil {
		return reqs
	}

	for _, h := range hcpList.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "instance",
				Namespace: h.Namespace,
			},
		})
	}
	return reqs
}

func (e *enqueueRequestForHostedControlPlane) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForHostedControlPlane) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.ObjectOld, reqs)
	e.mapAndEnqueue(q, evt.ObjectNew, reqs)
}

func (e *enqueueRequestForHostedControlPlane) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForHostedControlPlane) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}
