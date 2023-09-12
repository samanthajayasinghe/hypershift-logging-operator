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
	"fmt"
	"reflect"

	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
)

// HyperShiftLogForwarderReconciler reconciles a HyperShiftLogForwarder object
type HyperShiftLogForwarderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
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

	logForwarderTemplate := new(hlov1alpha1.ClusterLogForwarderTemplate)

	if err := r.Get(ctx, req.NamespacedName, logForwarderTemplate); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Find all relevant hostedcontrolplanes
	hcpList, err := r.GetHostedControlPlanes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, hcp := range hcpList {

		r.log.Info("Fetching kubeconfig HostedControlPlane", "namespace", hcp.Namespace, "name", hcp.Name)

		logForwarderList := new(loggingv1.ClusterLogForwarderList)
		if err := r.List(ctx, logForwarderList, &client.ListOptions{
			Namespace: hcp.Namespace,
		}); err != nil {
			r.log.Error(err, "error when listing")
		}

		if len(logForwarderList.Items) == 0 {
			// Create a logForwarder if none exists
			if err := r.CreateLogForwarder(ctx, logForwarderTemplate, hcp.Namespace); err != nil {
				r.log.Error(err, "error when creating object")
			}
			r.log.V(0).Info("Creating logForwarder", "namespace", hcp.Namespace)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HyperShiftLogForwarderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hlov1alpha1.HyperShiftLogForwarder{}).
		Complete(r)
}

func (r *HyperShiftLogForwarderReconciler) AddHostedClusters(mgr ctrl.Manager, clientContexts ...string) map[string]cluster.Cluster {
	clusters := map[string]cluster.Cluster{}

	for _, v := range clientContexts {
		conf, err := config.GetConfigWithContext(v)
		r.log.V(1).Error(err, "get client config fail", "context", v)

		c, err := cluster.New(conf)
		r.log.V(1).Error(err, "new cluster fail", "context", v)

		err = mgr.Add(c)
		r.log.V(1).Error(err, "add cluster in manager", "context", v)

		clusters[v] = c
	}
	return clusters
}

func (r *HyperShiftLogForwarderReconciler) GetHostedControlPlanes(
	ctx context.Context,
) ([]hyperv1beta1.HostedControlPlane, error) {

	hcpList := new(hyperv1beta1.HostedControlPlaneList)
	if err := r.List(ctx, hcpList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	return hcpList.Items, nil
}

// CreateLogForwarder creates a logForwarder provided a logForwarderTemplate and a namespace
func (r *HyperShiftLogForwarderReconciler) CreateLogForwarder(
	ctx context.Context,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate,
	namespace string,
) error {

	if reflect.ValueOf(logForwarderTemplate.Spec.Template).IsZero() {
		return fmt.Errorf("empty log forwarder template: %v", logForwarderTemplate.Spec.Template)
	}

	clusterLogForwarder := new(loggingv1.ClusterLogForwarder)
	clusterLogForwarder.Name = logForwarderTemplate.Name
	clusterLogForwarder.Namespace = namespace
	clusterLogForwarder.Spec = logForwarderTemplate.Spec.Template

	r.log.V(0).Info("Creating logForwarder", "namespace", clusterLogForwarder.Namespace, "name", clusterLogForwarder.Name)
	if err := r.Create(ctx, clusterLogForwarder); err != nil {
		return err
	}

	return nil
}
