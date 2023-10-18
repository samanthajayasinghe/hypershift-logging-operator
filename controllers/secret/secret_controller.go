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

package secret

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/openshift/hypershift-logging-operator/pkg/constants"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
)

// SecretReconciler reconciles a Secret object from the hosted cluster
type SecretReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	MCClient     client.Client
	HCPNamespace string
	log          logr.Logger
}

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	r.log = ctrllog.FromContext(ctx).WithName("hostedcluster-secret-controller")
	r.log.V(1).Info("start reconcile", "Name", req.NamespacedName)
	secret := &corev1.Secret{}

	if r.HCPNamespace == "" {
		return ctrl.Result{}, nil
	}

	// Getting the hosted cluster secret
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Getting the management cluster secret
	hcpSecret := &corev1.Secret{}

	hcpSecretFound := false
	err := r.MCClient.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name, Namespace: r.HCPNamespace}, hcpSecret)
	if err != nil && errors.IsNotFound(err) {
		hcpSecretFound = false
	} else if err == nil {
		hcpSecretFound = true
	} else {
		return ctrl.Result{}, err
	}

	secretFinalizerName := constants.ManagedLoggingFinalizer + "/secret"
	if !secret.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(secret, secretFinalizerName) {

			//remove HCP NS secret
			if hcpSecretFound {
				if err := r.MCClient.Delete(ctx, hcpSecret); err != nil {
					return ctrl.Result{}, err
				}
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(secret, secretFinalizerName)
			if err := r.Update(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
		}
		r.log.V(1).Info("Secret deleted", "UID", secret.UID, "Name", secret.Name)
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil

	}

	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if !controllerutil.ContainsFinalizer(secret, secretFinalizerName) {
		controllerutil.AddFinalizer(secret, secretFinalizerName)
		if err := r.Update(ctx, secret); err != nil {
			return ctrl.Result{}, err
		}
	}

	// copy the secret from hosted cluster to management cluster
	if err := r.updateOrCreateSecret(secret, ctx, hcpSecretFound); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateOrCreateSecret creates or update secret in HCP namespace
func (r *SecretReconciler) updateOrCreateSecret(
	secret *corev1.Secret,
	ctx context.Context,
	hcpSecretFound bool,
) error {
	secret.Namespace = r.HCPNamespace

	if hcpSecretFound {
		if err := r.MCClient.Update(ctx, secret); err != nil {
			return err
		}
	} else {
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		//Adding backup exclude label
		secret.Labels[constants.BackupExcludeLabel] = "true"

		if err := r.MCClient.Create(ctx, secret); err != nil {
			return err
		}
	}
	return nil
}
