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

// HostedCluster keeps hosted cluster info
type HostedCluster struct {
	Cluster      cluster.Cluster
	ClusterId    string
	HCPNamespace string
}

// HyperShiftLogForwarderReconciler reconciles a HyperShiftLogForwarder object
type HyperShiftLogForwarderReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	MCClint      client.Client
	HCPNamespace string
	log          logr.Logger
}

// NewLogForwarderReconcilerReconciler ...
func NewHyperShiftLogForwarderReconciler(mgr ctrl.Manager, hostedClusters map[string]HostedCluster) (*HyperShiftLogForwarderReconciler, error) {
	r := HyperShiftLogForwarderReconciler{
		Scheme: mgr.GetScheme(),
	}

	for _, hostedCluster := range hostedClusters {
		r := HyperShiftLogForwarderReconciler{
			Client:       hostedCluster.Cluster.GetClient(),
			Scheme:       mgr.GetScheme(),
			MCClint:      mgr.GetClient(),
			HCPNamespace: hostedCluster.HCPNamespace,
		}

		err := ctrl.NewControllerManagedBy(mgr).
			For(&hlov1alpha1.HyperShiftLogForwarder{}).
			Watches(
				source.NewKindWithCache(&hlov1alpha1.HyperShiftLogForwarder{}, hostedCluster.Cluster.GetCache()),
				&handler.EnqueueRequestForObject{},
			).
			Complete(&r)
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

	hlf := &hlov1alpha1.HyperShiftLogForwarder{}
	if err := r.Get(ctx, req.NamespacedName, hlf); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.log.V(1).Info("found HLF", "name", hlf.Name, "Namespace", hlf.Namespace)

	// Update CLF goes here

	return ctrl.Result{}, nil
}
