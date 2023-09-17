/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hypershiftlogforwarder

import (
	"context"

	"github.com/go-logr/logr"
	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// HyperShiftLogForwarderReconciler reconciles a HyperShiftLogForwarder object
type HyperShiftLogForwarderReconciler struct {
	client.Client
	Clients map[string]client.Client
	Scheme  *runtime.Scheme
	log     logr.Logger
}

// NewLogForwarderReconcilerReconciler ...
func NewHyperShiftLogForwarderReconciler(mgr ctrl.Manager, clusters map[string]cluster.Cluster) (*HyperShiftLogForwarderReconciler, error) {
	r := HyperShiftLogForwarderReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Clients: map[string]client.Client{
			//"main": mgr.GetClient(),
		},
	}
	for name, cluster := range clusters {
		r.Clients[name] = cluster.GetClient()
	}

	for _, cluster := range clusters {
		err := ctrl.NewControllerManagedBy(mgr).
			For(&hlov1alpha1.HyperShiftLogForwarder{}).
			Watches(
				source.NewKindWithCache(&hlov1alpha1.HyperShiftLogForwarder{}, cluster.GetCache()),
				&handler.EnqueueRequestForObject{},
			).
			Complete(&HyperShiftLogForwarderReconciler{
				Client:  mgr.GetClient(),
				Clients: r.Clients,
			})
		if err != nil {
			return &r, err
		}
	}

	return &r, nil
}

//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=hypershiftlogforwarders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=hypershiftlogforwarders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=hypershiftlogforwarders/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HyperShiftLogForwarder object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *HyperShiftLogForwarderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx).WithName("controller")

	hyperShiftLogForwarder := new(hlov1alpha1.HyperShiftLogForwarder)

	if err := r.Get(ctx, req.NamespacedName, hyperShiftLogForwarder); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// create pod in sub cluster
	/*for name, c := range r.Clients {

		//found := &hlov1alpha1.HyperShiftLogForwarder{}
		//err := c.Get(ctx, client.ObjectKeyFromObject(pod), found)
		//if err != nil && errors.IsNotFound(err) {

		/*subHyperShiftLogForwarder := new(hlov1alpha1.HyperShiftLogForwarder)
		subHyperShiftLogForwarder.Namespace = hyperShiftLogForwarder.Namespace
		subHyperShiftLogForwarder.Name = hyperShiftLogForwarder.Name
		subHyperShiftLogForwarder.Spec = hyperShiftLogForwarder.Spec

		activeHyperShiftLogForwarder := &hlov1alpha1.HyperShiftLogForwarder{}
		err := c.Get(ctx, client.ObjectKeyFromObject(subHyperShiftLogForwarder), activeHyperShiftLogForwarder)
		if errors.IsNotFound(err) {
			r.log.V(1).Info("creating hyperShiftLogForwarder: sub cluster", "Name", name, "namespace", subHyperShiftLogForwarder.Namespace, "name", subHyperShiftLogForwarder.Name)
			err := c.Create(ctx, subHyperShiftLogForwarder)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			r.log.V(1).Info("updating hyperShiftLogForwarder: sub cluster", "Name", name, "namespace", subHyperShiftLogForwarder.Namespace, "name", subHyperShiftLogForwarder.Name)
			activeHyperShiftLogForwarder.Spec = hyperShiftLogForwarder.Spec
			err := c.Update(ctx, activeHyperShiftLogForwarder)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		//err := c.Get(ctx, client.ObjectKeyFromObject(subHyperShiftLogForwarder), activeHyperShiftLogForwarder)

	}*/

	r.log.V(1).Info("updating hyperShiftLogForwarder: hcp cluster", "namespace", req.Namespace, "name", req.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HyperShiftLogForwarderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	co := ctrl.NewControllerManagedBy(mgr).
		For(&hlov1alpha1.HyperShiftLogForwarder{}).
		Complete(&HyperShiftLogForwarderReconciler{
			Client:  mgr.GetClient(),
			Clients: r.Clients,
		})

	return co
}

/*func (r *HyperShiftLogForwarderReconciler) findObjectsFormMap(configMap hlov1alpha1.HyperShiftLogForwarder) []reconcile.Request {
    attachedConfigDeployments := &appsv1.ConfigDeploymentList{}
    listOps := &client.ListOptions{
        FieldSelector: fields.OneTermEqualSelector(configMapField, configMap.GetName()),
        Namespace:     configMap.GetNamespace(),
    }
    err := r.List(context.TODO(), attachedConfigDeployments, listOps)
    if err != nil {
        return []reconcile.Request{}
    }

    requests := make([]reconcile.Request, len(attachedConfigDeployments.Items))
    for i, item := range attachedConfigDeployments.Items {
        requests[i] = reconcile.Request{
            NamespacedName: types.NamespacedName{
                Name:      item.GetName(),
                Namespace: item.GetNamespace(),
            },
        }
    }
    return requests
}*/
