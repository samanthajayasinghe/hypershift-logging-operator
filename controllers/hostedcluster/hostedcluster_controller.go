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

package hostedcluster

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/controllers/hypershiftlogforwarder"
	"github.com/openshift/hypershift-logging-operator/pkg/hostedcluster"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var hostedClusters = map[string]hypershiftlogforwarder.HostedCluster{}

// HostedClusterReconciler reconciles a HostedCluster object
type HostedClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
	Mgr    ctrl.Manager
}

// +kubebuilder:rbac:groups=hypershift.openshift.io,resources=hostedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hypershift.openshift.io,resources=hostedclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hypershift.openshift.io,resources=hostedclusters/finalizers,verbs=update
// Reconcile actions for newly created hosted clusters and deleted hosted clusters.
//
// If it's a new hosted cluster, Reconciler creates a new manager
// with hypershift-log-forwarder controller and starts the sub manager.
//
// If it's a deleted cluster, Reconciler cancel the sub-manager context,
// which leads to stopping the hypershift-log-forwarder controller and sub-manager
func (r *HostedClusterReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {

	log := logr.Logger{}.WithName("hostedcluster-controller")

	hostedCluster := &hyperv1beta1.HostedCluster{}
	if err := r.Get(ctx, req.NamespacedName, hostedCluster); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	found := false
	err := r.Get(ctx, req.NamespacedName, hostedCluster)
	if err != nil && errors.IsNotFound(err) {
		found = false
	} else if err == nil {
		found = true
	} else {
		return ctrl.Result{}, err
	}

	currentHostedCluster, exist := hostedClusters[hostedCluster.Name]

	hcpNamespace := fmt.Sprintf("%s-%s", hostedCluster.Namespace, hostedCluster.Name)
	hcpName := hostedCluster.Name

	if !exist {
		// check hosted cluster status, if it's new created and ready, start the reconcile
		newReadyCluster := hostedcluster.IsReadyHostedCluster(*hostedCluster)
		if newReadyCluster {
			restConfig, err := hostedcluster.BuildGuestKubeConfig(r.Client, hcpNamespace, r.log)
			if err != nil {
				log.Error(err, "getting guest cluster kubeconfig")
			}

			hsCluster, err := cluster.New(restConfig)
			if err != nil {
				log.Error(err, "creating guest cluster kubeconfig")
			}
			clusterScheme := hsCluster.GetScheme()
			utilruntime.Must(hyperv1beta1.AddToScheme(clusterScheme))
			utilruntime.Must(v1alpha1.AddToScheme(clusterScheme))

			ctx := context.Background()
			ctx, cancelFunc := context.WithCancel(ctx)

			newHostedCluster := hypershiftlogforwarder.HostedCluster{
				Cluster:      hsCluster,
				HCPNamespace: hcpNamespace,
				ClusterName:  hostedCluster.Name,
				Context:      &ctx,
				CancelFunc:   &cancelFunc,
			}
			hostedClusters[hcpName] = newHostedCluster
			rhc := hypershiftlogforwarder.HyperShiftLogForwarderReconciler{
				Client:       hsCluster.GetClient(),
				Scheme:       r.Scheme,
				MCClient:     r.Client,
				HCPNamespace: hcpNamespace,
			}

			leaderElectionID := fmt.Sprintf("%s.logging.managed.openshift.io", hostedCluster.Name)
			mgrHostedCluster, err := ctrl.NewManager(newHostedCluster.Cluster.GetConfig(), ctrl.Options{
				Scheme:                 r.Scheme,
				HealthProbeBindAddress: "",
				LeaderElection:         false,
				MetricsBindAddress:     "0",
				LeaderElectionID:       leaderElectionID,
			})

			go func() {
				err = ctrl.NewControllerManagedBy(mgrHostedCluster).
					Named(hostedCluster.Name).
					For(&v1alpha1.HyperShiftLogForwarder{}).
					Complete(&rhc)

				r.log.Info("starting HostedCluster manager", "Name", hostedCluster.Name)
				if err := mgrHostedCluster.Start(ctx); err != nil {
					r.log.Error(err, "problem running HostedCluster manager", "Name", hostedCluster.Name)
				}

			}()

			return ctrl.Result{}, nil
		}

	} else {
		if !found {
			//if it's deleted, stop the reconcile
			r.log.V(1).Info("testing", "found", found)
			cancelFunc := *currentHostedCluster.CancelFunc
			cancelFunc()
			r.log.V(1).Info("finished context")
		}
	}

	return ctrl.Result{}, nil
}

func eventPredicates() predicate.Predicate {
	return predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HostedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hyperv1beta1.HostedCluster{}).
		WithEventFilter(eventPredicates()).
		Complete(r)
}
