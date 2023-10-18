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
	hypershiftsecret "github.com/openshift/hypershift-logging-operator/controllers/secret"
	constants "github.com/openshift/hypershift-logging-operator/pkg/constants"
	"github.com/openshift/hypershift-logging-operator/pkg/hostedcluster"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	clusterScheme  = runtime.NewScheme()
	hostedClusters = map[string]hypershiftlogforwarder.HostedCluster{}
)

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
	found := false
	err := r.Get(ctx, req.NamespacedName, hostedCluster)
	if err != nil && errors.IsNotFound(err) {
		found = false
	} else if err == nil {
		found = true
	} else {
		return ctrl.Result{}, err
	}

	_, exist := hostedClusters[req.NamespacedName.Name]

	hcpNamespace := fmt.Sprintf("%s-%s", hostedCluster.Namespace, hostedCluster.Name)
	isReadyCluster := hostedcluster.IsReadyHostedCluster(*hostedCluster)

	if !exist {
		// check hosted cluster status, if it's new created and ready, start the reconcile

		if isReadyCluster {
			restConfig, err := hostedcluster.BuildGuestKubeConfig(r.Client, hcpNamespace, r.log)
			if err != nil {
				log.Error(err, "getting guest cluster kubeconfig")
				return ctrl.Result{}, err
			}

			hsCluster, err := cluster.New(restConfig)
			if err != nil {
				log.Error(err, "creating guest cluster kubeconfig")
				return ctrl.Result{}, err
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
				Context:      ctx,
				CancelFunc:   cancelFunc,
			}
			hostedClusters[req.NamespacedName.Name] = newHostedCluster
			rhc := hypershiftlogforwarder.HyperShiftLogForwarderReconciler{
				Client:       hsCluster.GetClient(),
				Scheme:       clusterScheme,
				MCClient:     r.Client,
				HCPNamespace: hcpNamespace,
			}

			rHostedClusterSecret := hypershiftsecret.SecretReconciler{
				Client:       hsCluster.GetClient(),
				Scheme:       clusterScheme,
				MCClient:     r.Client,
				HCPNamespace: hcpNamespace,
			}

			leaderElectionID := fmt.Sprintf("%s.logging.managed.openshift.io", hostedCluster.Name)

			mgrHostedCluster, err := ctrl.NewManager(restConfig, ctrl.Options{
				Scheme:                 clusterScheme,
				HealthProbeBindAddress: "",
				LeaderElection:         false,
				MetricsBindAddress:     "0",
				LeaderElectionID:       leaderElectionID,
				Namespace:              constants.HLFWatchedNamespace,
			})

			if err != nil {
				log.Error(err, "creating new sub manager")
				return ctrl.Result{}, err
			}

			//Adding hosted cluster to sub manger
			err = mgrHostedCluster.Add(hsCluster)
			if err != nil {
				log.Error(err, "Adding hosted cluster runnable to sub manager")
				return ctrl.Result{}, err
			}

			go func() {
				//Add hypershift log forwarder to sub manager
				err = ctrl.NewControllerManagedBy(mgrHostedCluster).
					Named(hostedCluster.Name).
					For(&v1alpha1.HyperShiftLogForwarder{}).
					WithEventFilter(eventPredicates()).
					Complete(&rhc)

				if err != nil {
					r.log.Error(err, "problem adding hypershift log forwarder controller to sub manager", "Name", hostedCluster.Name)
				}

				// Add hosted cluster secret to sub manager
				controllerName := fmt.Sprintf("secret_%s", hostedCluster.Name)
				err = ctrl.NewControllerManagedBy(mgrHostedCluster).
					Named(controllerName).
					For(&corev1.Secret{}).
					Complete(&rHostedClusterSecret)

				if err != nil {
					r.log.Error(err, "problem adding secret controller to sub manager", "Name", hostedCluster.Name)
				}

				r.log.Info("starting HostedCluster manager", "Name", hostedCluster.Name)
				if err := mgrHostedCluster.Start(ctx); err != nil {
					r.log.Error(err, "problem running HostedCluster manager", "Name", hostedCluster.Name)
				}

			}()

			return ctrl.Result{}, nil
		}

	} else {
		//Stop the controller when cluster is not ready or deleted

		r.log.V(1).Info("Stop existing managers", "ready cluster", isReadyCluster, "found", found)
		validKubeConfig, _ := hostedcluster.ValidateKubeConfig(r.Client, hcpNamespace)

		if !isReadyCluster || !found || !validKubeConfig {
			cancelFunc := hostedClusters[req.NamespacedName.Name].CancelFunc
			cancelFunc()
			r.log.V(1).Info("stop the manager", "controller name", hostedClusters[req.NamespacedName.Name])

			//delete hosted cluster from the map since it may create / active again
			delete(hostedClusters, req.NamespacedName.Name)
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
