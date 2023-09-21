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
	"strconv"
	"strings"

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
	"github.com/openshift/hypershift-logging-operator/pkg/hostedcluster"
)

const (
	// Same as the CLO, we set the CLFT as signleton here
	singletonName = "instance"
	// ManagedLogForwardTemplatePrefix prefix to identify the entries in the array are managed by template
	ManagedLogForwardTemplatePrefix = "openshift-sre"
	TemplateFinalizer               = "logging.managed.openshift.io"
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

	if req.NamespacedName.Name != singletonName {
		err := fmt.Errorf("clusterLogForwarderTemplate name must be '%s'", singletonName)
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
		controllerutil.RemoveFinalizer(template, TemplateFinalizer)
		err = r.Client.Update(ctx, template)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		deletion = false
		controllerutil.AddFinalizer(template, TemplateFinalizer)
		err = r.Client.Update(ctx, template)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	clusterLogForwarder := &loggingv1.ClusterLogForwarder{}

	for _, hcp := range hcpList {
		found := false
		err := r.Get(ctx, types.NamespacedName{Name: "instance", Namespace: hcp.Namespace}, clusterLogForwarder)
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
				clusterLogForwarder = r.CleanUpClusterLogForwarder(clusterLogForwarder)
				err = r.Update(ctx, clusterLogForwarder)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}
		if !deletion {
			// For any reconcile loop, clean up the CLF for entries from CLFT, and build from new CLFT
			clusterLogForwarder.Name = "instance"
			clusterLogForwarder.Namespace = hcp.Namespace

			//TODO: Need to fix the serviceaccount name here once we know how to use it
			//clusterLogForwarder.Spec.ServiceAccountName = template.Spec.Template.ServiceAccountName

			r.CleanUpClusterLogForwarder(clusterLogForwarder)
			r.BuildInputs(clusterLogForwarder, template)
			r.BuildOutputs(clusterLogForwarder, template)
			r.BuildPipelines(clusterLogForwarder, template)

			// Found existing CLF, update with new CLFT
			if found {
				if err := r.Update(context.TODO(), clusterLogForwarder); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			// No CLF found, create with the CLFT entries
			if !found {
				if err := r.Create(ctx, clusterLogForwarder); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}

// BuildInputs builds the input array from the template and adds to the clean log forwarder
func (r *ClusterLogForwarderTemplateReconciler) BuildInputs(clusterLogForwarder *loggingv1.ClusterLogForwarder,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate) {

	// Build the input from template
	if len(logForwarderTemplate.Spec.Template.Inputs) < 1 {
		r.log.V(3).Info("there is no input defined, skipping...")
	} else {
		for _, tInput := range logForwarderTemplate.Spec.Template.Inputs {
			if !strings.Contains(tInput.Name, ManagedLogForwardTemplatePrefix) {
				tInput.Name = ManagedLogForwardTemplatePrefix + "-" + tInput.Name
			}
			clusterLogForwarder.Spec.Inputs = append(clusterLogForwarder.Spec.Inputs, tInput)
		}
	}
}

// BuildOutputs builds the output array from the template and adds to the clean log forwarder
func (r *ClusterLogForwarderTemplateReconciler) BuildOutputs(clusterLogForwarder *loggingv1.ClusterLogForwarder,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate) {

	// Build the pipelines from template
	if len(logForwarderTemplate.Spec.Template.Outputs) < 1 {
		r.log.V(3).Info("there is no output defined, skipping...")
	} else {
		for _, tOutput := range logForwarderTemplate.Spec.Template.Outputs {
			if !strings.Contains(tOutput.Name, ManagedLogForwardTemplatePrefix) {
				tOutput.Name = ManagedLogForwardTemplatePrefix + "-" + tOutput.Name
			}
			clusterLogForwarder.Spec.Outputs = append(clusterLogForwarder.Spec.Outputs, tOutput)
		}
	}
}

// BuildPipelines builds the pipeline array from the template and adds to the clean log forwarder
func (r *ClusterLogForwarderTemplateReconciler) BuildPipelines(clusterLogForwarder *loggingv1.ClusterLogForwarder,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate) {

	// Build the pipelines from template
	if len(logForwarderTemplate.Spec.Template.Pipelines) < 1 {
		r.log.V(3).Info("there is no pipeline defined, skipping...")
	} else {
		autoGenName := "auto-generated-name"
		for x, tPpl := range logForwarderTemplate.Spec.Template.Pipelines {
			if tPpl.Name == "" {
				tPpl.Name = autoGenName + strconv.Itoa(x)
			}
			if !strings.Contains(tPpl.Name, ManagedLogForwardTemplatePrefix) {
				tPpl.Name = ManagedLogForwardTemplatePrefix + "-" + tPpl.Name
			}
			clusterLogForwarder.Spec.Pipelines = append(clusterLogForwarder.Spec.Pipelines, tPpl)
		}
	}
}

// CleanUpClusterLogForwarder clear the related entries when the template deleted
func (r *ClusterLogForwarderTemplateReconciler) CleanUpClusterLogForwarder(clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	newPipelines := clf.Spec.Pipelines[:0]
	for _, ppl := range clf.Spec.Pipelines {
		if !strings.Contains(ppl.Name, ManagedLogForwardTemplatePrefix) {
			newPipelines = append(newPipelines, ppl)
		}
	}

	newInputs := clf.Spec.Inputs[:0]
	for _, input := range clf.Spec.Inputs {
		if !strings.Contains(input.Name, ManagedLogForwardTemplatePrefix) {
			newInputs = append(newInputs, input)
		}
	}
	newOutputs := clf.Spec.Outputs[:0]
	for _, output := range clf.Spec.Outputs {
		if !strings.Contains(output.Name, ManagedLogForwardTemplatePrefix) {
			newOutputs = append(newOutputs, output)
		}
	}
	clf.Spec.Pipelines = newPipelines
	clf.Spec.Inputs = newInputs
	clf.Spec.Outputs = newOutputs
	return clf
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
