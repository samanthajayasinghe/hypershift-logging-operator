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

package clusterlogforwardertemplate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/pkg/clusterlogforwarder"
	"github.com/openshift/hypershift-logging-operator/pkg/consts"
	"github.com/openshift/hypershift-logging-operator/pkg/hostedcluster"
)

// ClusterLogForwarderTemplateReconciler reconciles a ClusterLogForwarderTemplate object
type ClusterLogForwarderTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=logging.managed.openshift.io,resources=clusterlogforwardertemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterLogForwarderTemplateReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx).WithName("controller")

	if req.NamespacedName.Name != consts.SingletonName {
		err := fmt.Errorf("clusterLogForwarderTemplate name must be '%s'", consts.SingletonName)
		r.log.V(1).Error(err, "")
		return ctrl.Result{}, err
	}

	hcpList, err := hostedcluster.GetHostedControlPlanes(r.Client, ctx, false)
	if err != nil {
		return ctrl.Result{}, err
	}

	template := &hlov1alpha1.ClusterLogForwarderTemplate{}
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deletion := false

	if !template.DeletionTimestamp.IsZero() {
		deletion = true
		controllerutil.RemoveFinalizer(template, consts.ManagedLoggingFinalizer)
		err = r.Client.Update(ctx, template)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		deletion = false
		controllerutil.AddFinalizer(template, consts.ManagedLoggingFinalizer)
		err = r.Client.Update(ctx, template)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	clf := &loggingv1.ClusterLogForwarder{}

	for _, hcp := range hcpList {
		found := false
		err := r.Get(ctx, types.NamespacedName{Name: "instance", Namespace: hcp.Namespace}, clf)
		if err != nil && errors.IsNotFound(err) {
			found = false
		} else if err == nil {
			found = true
		} else {
			return ctrl.Result{}, err
		}

		if deletion {
			// If CLFT is being deleted, and no CLF found, skip
			if !found {
				return ctrl.Result{}, nil
			}
			// If CLFT is being deleted, and CLF found, clean up the CLF with all the CLFT entries
			if found {
				clusterlogforwarder.CleanUpClusterLogForwarder(clf, consts.ProviderManagedRuleNamePrefix)
				err = r.Update(ctx, clf)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}
		if !deletion {
			// For any reconcile loop, clean up the CLF for entries from CLFT, and build from new CLFT
			clf.Name = "instance"
			clf.Namespace = hcp.Namespace

			//TODO: Need to fix the serviceaccount name here once we know how to use it
			//clusterLogForwarder.Spec.ServiceAccountName = template.Spec.Template.ServiceAccountName

			clusterlogforwarder.CleanUpClusterLogForwarder(clf, consts.ProviderManagedRuleNamePrefix)
			clf = clusterlogforwarder.BuildInputsFromTemplate(template, clf)
			clf = clusterlogforwarder.BuildOutputsFromTemplate(template, clf)
			clf = clusterlogforwarder.BuildPipelinesFromTemplate(template, clf)

			// Found existing CLF, update with new CLFT
			if found {
				if err := r.Update(context.TODO(), clf); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			// No CLF found, create with the CLFT entries
			if !found {
				if err := r.Create(ctx, clf); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
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
func (r *ClusterLogForwarderTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hlov1alpha1.ClusterLogForwarderTemplate{}).
		WithEventFilter(eventPredicates()).
		Complete(r)
}
