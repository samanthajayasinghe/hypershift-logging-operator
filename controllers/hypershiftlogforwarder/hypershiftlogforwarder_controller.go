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
	"strings"

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

	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"

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
	illegalNameCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "IllegalName",
		Message: fmt.Sprintf("The Name contains the SRE preserved string %s", constants.ProviderManagedRuleNamePrefix),
	}
	incorrectInstanceNameCondition = loggingv1.Condition{
		Type:    "Degraded",
		Status:  "True",
		Reason:  "NonSupportResourceName",
		Message: fmt.Sprintf("The name of the HyperShiftLogForwarder must be '%s'", constants.SingletonName),
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
	err := r.MCClient.Get(context.TODO(), types.NamespacedName{Name: "instance", Namespace: r.HCPNamespace}, clf)
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

			//removes all the user managed forwarder rules from CLF
			clusterlogforwarder.CleanUpClusterLogForwarder(clf, constants.CustomerManagedRuleNamePrefix)

			//Update or create clf
			if err := r.updateOrCreateCLF(clf, instance, ctx, clfFound); err != nil {
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, constants.ManagedLoggingFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
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
	r.log.V(1).Info("Found new or update HLF", "UID", instance.UID, "Name", instance.Name)

	if req.NamespacedName.Name != constants.SingletonName {
		r.log.V(3).Info("hyperShiftLogForwarder is singleton and name should be 'instance'")
		instance.Status.Conditions.SetCondition(incorrectInstanceNameCondition)
		err = r.Status().Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Do not need to validate the inputs from HLF since we build it as fixed format for now
	//if err = r.ValidateInputs(instance); err != nil {
	//	return ctrl.Result{}, err
	//}

	if err = r.ValidateOutputs(instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.ValidatePipelines(instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.ValidateFilters(instance); err != nil {
		return ctrl.Result{}, err
	}

	instance.Status.Conditions.RemoveCondition(illegalNameCondition.Type)
	instance.Status.Conditions.RemoveCondition(nonSupportInputTypeCondition.Type)
	instance.Status.Conditions.RemoveCondition(incorrectInstanceNameCondition.Type)
	instance.Status.Conditions.RemoveCondition(nonSupportFilterTypeCondition.Type)
	instance.Status.Conditions.RemoveCondition(nonSupportedInputRefCondition.Type)
	if err = r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateOrCreateCLF(clf, instance, ctx, clfFound); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// updateOrCreateCLF creates or update clf in HCP namespace
func (r *HyperShiftLogForwarderReconciler) updateOrCreateCLF(
	clf *loggingv1.ClusterLogForwarder,
	instance *v1alpha1.HyperShiftLogForwarder,
	ctx context.Context,
	clfFound bool,
) error {
	clf.Namespace = r.HCPNamespace
	clf.Name = "instance"
	clf = clusterlogforwarder.BuildInputsFromHLF(instance, clf)
	clf = clusterlogforwarder.BuildOutputsFromHLF(instance, clf)
	clf = clusterlogforwarder.BuildPipelinesFromHLF(instance, clf)
	clf = clusterlogforwarder.BuildFiltersFromHLF(instance, clf)

	if clfFound {
		if err := r.MCClient.Update(ctx, clf); err != nil {
			return err
		}
	} else {
		if err := r.MCClient.Create(ctx, clf); err != nil {
			return err
		}
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
		if strings.Contains(input.Name, constants.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the input name", constants.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the input name", constants.ProviderManagedRuleNamePrefix)
		}
	}
	return nil
}

// ValidateOutputs validates the HLF outputs
func (r *HyperShiftLogForwarderReconciler) ValidateOutputs(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, output := range hlf.Spec.Outputs {
		if strings.Contains(output.Name, constants.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the output name", constants.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the output name", constants.ProviderManagedRuleNamePrefix)
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
		if strings.Contains(ppl.Name, constants.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the pipeline name", constants.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the pipeline name", constants.ProviderManagedRuleNamePrefix)
		}
	}
	return nil
}

// ValidateFilters validates the HLF filters
func (r *HyperShiftLogForwarderReconciler) ValidateFilters(hlf *v1alpha1.HyperShiftLogForwarder) error {
	for _, f := range hlf.Spec.Filters {
		if strings.Contains(f.Name, constants.ProviderManagedRuleNamePrefix) {
			r.log.V(3).Info(fmt.Sprintf("preserved string %s cannot be set to the filter name", constants.ProviderManagedRuleNamePrefix))
			hlf.Status.Conditions.SetCondition(illegalNameCondition)
			if err := r.Status().Update(context.TODO(), hlf); err != nil {
				return err
			}
			return fmt.Errorf("preserved string %s cannot be set to the filter name", constants.ProviderManagedRuleNamePrefix)
		}
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

func (r *HyperShiftLogForwarderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hyperv1beta1.HostedCluster{}).
		// WithEventFilter(eventPredicates()).
		Complete(r)
}
