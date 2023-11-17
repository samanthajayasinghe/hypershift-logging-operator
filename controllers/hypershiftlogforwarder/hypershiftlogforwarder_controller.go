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

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/pkg/clusterlogforwarder"
	"github.com/openshift/hypershift-logging-operator/pkg/constants"
)

var (
	nonSupportInputTypeCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "NonSupportedInputType",
		Message: "The input supports only the audit type",
	}
	nonSupportedInputRefCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "NonSupportedInputRef",
		Message: fmt.Sprintf("The input ref can be %s only for the current release", clusterlogforwarder.InputHTTPServerName),
	}
	nonSupportFilterTypeCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "NonSupportedFilterType",
		Message: "The filter supports only the kubeAPIAudit type",
	}
	hostedClusters = map[string]HostedCluster{}
)

// HostedCluster keeps hosted cluster info
type HostedCluster struct {
	Cluster      cluster.Cluster
	ClusterId    string
	ClusterName  string
	HCPNamespace string
	Context      context.Context
	CancelFunc   context.CancelFunc
}

// HyperShiftLogForwarderReconciler reconciles a HyperShiftLogForwarder object
type HyperShiftLogForwarderReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	MCClient     client.Client
	HCPNamespace string
	log          logr.Logger
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HyperShiftLogForwarderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	r.log = ctrllog.FromContext(ctx).WithName("hyperShiftLogForwarder-controller")
	r.log.V(1).Info("start reconcile", "Name", req.NamespacedName)
	instance := &v1alpha1.HyperShiftLogForwarder{}

	if r.HCPNamespace == "" {
		return ctrl.Result{}, nil
	}

	// Getting the hlf
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Getting the clf
	clf := &loggingv1.ClusterLogForwarder{}

	clfFound := false
	err := r.MCClient.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: r.HCPNamespace}, clf)
	if err != nil && errors.IsNotFound(err) {
		clfFound = false
	} else if err == nil {
		clfFound = true
	} else {
		return ctrl.Result{}, err
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(instance, constants.ManagedLoggingFinalizer) {
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, constants.ManagedLoggingFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			// delete the CLF which created by the HLF
			if clfFound {
				if err = r.MCClient.Delete(ctx, clf); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		r.log.V(1).Info("HLF deleted", "UID", instance.UID, "Name", instance.Name)
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if !controllerutil.ContainsFinalizer(instance, constants.ManagedLoggingFinalizer) {
		controllerutil.AddFinalizer(instance, constants.ManagedLoggingFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}
	r.log.V(3).Info("Found new or update HLF", "UID", instance.UID, "Name", instance.Name)

	// Do not need to validate the inputs from HLF since we build it as fixed format for now
	//if err = r.ValidateInputs(instance); err != nil {
	//	return ctrl.Result{}, err
	//}

	if err = r.ValidatePipelines(instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.ValidateFilters(instance); err != nil {
		return ctrl.Result{}, err
	}

	instance.Status.Conditions.RemoveCondition(nonSupportInputTypeCondition.Type)
	instance.Status.Conditions.RemoveCondition(nonSupportFilterTypeCondition.Type)
	instance.Status.Conditions.RemoveCondition(nonSupportedInputRefCondition.Type)
	if err = r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.refreshCLF(clf, instance, ctx, clfFound); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HyperShiftLogForwarderReconciler) buildClusterLogForwarder(instance *v1alpha1.HyperShiftLogForwarder,
) *loggingv1.ClusterLogForwarder {

	clf := &loggingv1.ClusterLogForwarder{}

	clf.Name = instance.Name
	clf.Namespace = r.HCPNamespace

	clfBuilder := clusterlogforwarder.ClusterLogForwarderBuilder{
		Clf: clf,
		Hlf: instance,
	}
	labels := map[string]string{
		"hcp_namespace": r.HCPNamespace,
	}

	clfBuilder.BuildInputsFromHLF().
		BuildOutputsFromHLF().
		BuildPipelinesFromHLF(labels).
		BuildFiltersFromHLF()

	return clfBuilder.Clf
}

// updateOrCreateCLF creates or update clf in HCP namespace
func (r *HyperShiftLogForwarderReconciler) refreshCLF(
	oldClf *loggingv1.ClusterLogForwarder,
	instance *v1alpha1.HyperShiftLogForwarder,
	ctx context.Context,
	clfFound bool,
) error {

	newClf := r.buildClusterLogForwarder(instance)

	if clfFound {
		if reflect.DeepEqual(newClf.Spec, oldClf.Spec) {
			return nil
		} else {
			err := r.MCClient.Delete(ctx, oldClf)
			if err != nil {
				return err
			}
		}
	}

	err := r.MCClient.Create(ctx, newClf)
	if err != nil {
		return err
	}
	return nil
}

// ValidateInputs validates HLF inputs
func (r *HyperShiftLogForwarderReconciler) ValidateInputs(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, input := range hlf.Spec.Inputs {
		if input.Infrastructure != nil || input.Application != nil {
			r.log.V(3).Info("support only audit log for HyperShiftLogForwarder")
			hlf.Status.Conditions.SetCondition(nonSupportInputTypeCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("support only audit log for HyperShiftLogForwarder")
		}
	}
	return nil
}

// ValidatePipelines validates the HLF pipelines
func (r *HyperShiftLogForwarderReconciler) ValidatePipelines(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, ppl := range hlf.Spec.Pipelines {
		for _, ir := range ppl.InputRefs {
			if ir != clusterlogforwarder.InputHTTPServerName {
				r.log.V(3).Info(fmt.Sprintf("the input can be '%s' only for the current release", clusterlogforwarder.InputHTTPServerName))
				hlf.Status.Conditions.SetCondition(nonSupportedInputRefCondition)
				if err := r.Status().Update(context.TODO(), hlf); err != nil {
					return err
				}
				return fmt.Errorf(fmt.Sprintf("support only %s as input ref for current release", clusterlogforwarder.InputHTTPServerName))
			}
			if ir == "application" || ir == "infrastructure" {
				r.log.V(3).Info("support only audit log for HyperShiftLogForwarder")
				hlf.Status.Conditions.SetCondition(nonSupportInputTypeCondition)
				if err := r.Status().Update(context.TODO(), hlf); err != nil {
					return err
				}
				return fmt.Errorf("support only audit log for HyperShiftLogForwarder")
			}
		}
	}
	return nil
}

// ValidateFilters validates the HLF filters
func (r *HyperShiftLogForwarderReconciler) ValidateFilters(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, f := range hlf.Spec.Filters {
		if f.Type != "kubeAPIAudit" {
			r.log.V(3).Info(fmt.Sprintf("support only kubeAPIAudit type of filter for HyperShiftLogForwarder"))
			hlf.Status.Conditions.SetCondition(nonSupportFilterTypeCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("support only kubeAPIAudit filter for HyperShiftLogForwarder")
		}
	}
	return nil
}
