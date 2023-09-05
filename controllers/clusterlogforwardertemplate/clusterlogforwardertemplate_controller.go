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
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"

	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
)

// ClusterLogForwarderTemplateReconciler reconciles a ClusterLogForwarderTemplate object
type ClusterLogForwarderTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterLogForwarderTemplateReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx).WithName("controller")

	logForwarderTemplate := new(hlov1alpha1.ClusterLogForwarderTemplate)

	if err := r.Get(ctx, req.NamespacedName, logForwarderTemplate); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Find all relevant hostedcontrolplanes
	hcpList, err := r.GetHostedControlPlanes(ctx, logForwarderTemplate)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure a LogForwarder exists for a hostedcontrolplane
	if err := r.ValidateLogForwarderInHostedControlPlanes(ctx, logForwarderTemplate, hcpList); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// GetHostedControlPlanes returns a list of all hostedcontrolplane resources in all namespaces.
// Basically does `oc get hostedcontrolplane -A`
func (r *ClusterLogForwarderTemplateReconciler) GetHostedControlPlanes(
	ctx context.Context,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate,
) ([]hyperv1beta1.HostedControlPlane, error) {

	hcpList := new(hyperv1beta1.HostedControlPlaneList)
	if err := r.List(ctx, hcpList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	return hcpList.Items, nil
}

// ValidateLogForwarderInHostedControlPlanes
func (r *ClusterLogForwarderTemplateReconciler) ValidateLogForwarderInHostedControlPlanes(
	ctx context.Context,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate,
	hcpList []hyperv1beta1.HostedControlPlane,
) error {

	// We want one logForwarder per namespace/HCP per logForwarderTemplate
	for _, hcp := range hcpList {
		r.log.V(1).Info("Validating HostedControlPlane", "namespace", hcp.Namespace, "name", hcp.Name)

		logForwarderList := new(loggingv1.ClusterLogForwarderList)
		if err := r.List(ctx, logForwarderList, &client.ListOptions{
			Namespace: hcp.Namespace,
		}); err != nil {
			return err
		}

		// If the HostedControlPlane is deleting, delete the logForwarder
		if !hcp.DeletionTimestamp.IsZero() {
			for _, logForwarder := range logForwarderList.Items {
				logForwarder := logForwarder
				r.log.V(0).Info("Deleting logForwarder", "namespace", logForwarder.Namespace, "name", logForwarder.Name)
				if err := r.Delete(ctx, &logForwarder); err != nil {
					return err
				}
			}

			continue
		}

		switch {
		case len(logForwarderList.Items) > 1:
			r.log.V(0).Info("found more than one matching logForwarder, deleting extras")
			found := false
			for _, logForwarder := range logForwarderList.Items {
				logForwarder := logForwarder
				// Make sure we only keep one matching logForwarder
				// If we've already found one that's matching, delete the extras
				if reflect.DeepEqual(logForwarder.Spec, logForwarderTemplate.Spec) {
					found = true
					continue
				}

				// If the logForwarder doesn't match, delete it
				r.log.V(0).Info("Deleting logForwarder", "namespace", logForwarder.Namespace, "name", logForwarder.Name)
				if err := r.Delete(ctx, &logForwarder); err != nil {
					return err
				}
			}

			if !found {
				if err := r.CreateLogForwarder(ctx, logForwarderTemplate, hcp.Namespace); err != nil {
					return err
				}
			}

		case len(logForwarderList.Items) == 1:
			if err := r.ReplaceLogForwarderSpec(ctx, &logForwarderList.Items[0], logForwarderTemplate); err != nil {
				return err
			}
		case len(logForwarderList.Items) == 0:
			// Create a logForwarder if none exists
			if err := r.CreateLogForwarder(ctx, logForwarderTemplate, hcp.Namespace); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateLogForwarder creates a logForwarder provided a logForwarderTemplate and a namespace
func (r *ClusterLogForwarderTemplateReconciler) CreateLogForwarder(
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

// ReplaceLogForwarderSpec effectively does a "kubectl replace" if the provided actual LogForwarder doesn't match the logForwarderTemplate
func (r *ClusterLogForwarderTemplateReconciler) ReplaceLogForwarderSpec(
	ctx context.Context,
	actual *loggingv1.ClusterLogForwarder,
	logForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate,
) error {
	if reflect.ValueOf(logForwarderTemplate.Spec.Template).IsZero() {
		return fmt.Errorf("empty log forwarder template: %v", logForwarderTemplate.Spec.Template)
	}

	if !reflect.DeepEqual(actual.Spec, *logForwarderTemplate.Spec.Template.DeepCopy()) {
		actual.Spec = *logForwarderTemplate.Spec.Template.DeepCopy()
		r.log.V(0).Info("Replacing LogForwarder", "namespace", actual.Namespace, "name", actual.Name)
		if err := r.Update(ctx, actual); err != nil {
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterLogForwarderTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hlov1alpha1.ClusterLogForwarderTemplate{}).
		Complete(r)
}
