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

package serviceaccount

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openshift/hypershift-logging-operator/pkg/constants"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAccountReconciler reconciles a service account object from the hosted cluster
type ServiceAccountReconciler struct {
	client.Client
	ClientSet    *kubernetes.Clientset
	Scheme       *runtime.Scheme
	MCClient     client.Client
	HCPNamespace string
	log          logr.Logger
}

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	r.log = ctrllog.FromContext(ctx).WithName("hostedcluster-service-account-controller")
	r.log.V(1).Info("start reconcile", "Name", req.NamespacedName)

	enabled, err := r.checkAuditLogEnabled(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If the audit log is not enabled, we skip the reconcile and retry in 10 minutes
	if !enabled {
		return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
	}

	serviceAccount := &corev1.ServiceAccount{}

	if r.HCPNamespace == "" {
		return ctrl.Result{}, nil
	}

	serviceAccountExists := false
	// Getting the hosted cluster service account
	apiContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = r.Get(
		apiContext,
		types.NamespacedName{Name: constants.MintServiceAccountName, Namespace: constants.MintServiceAccountNamespace},
		serviceAccount,
	)

	if err != nil && errors.IsNotFound(err) {
		serviceAccountExists = false
	} else if err == nil {
		serviceAccountExists = true
	} else {
		return ctrl.Result{RequeueAfter: constants.TokenRefreshDuration}, nil
	}

	if serviceAccountExists {
		r.log.V(1).Info("Found existing service account", "Name", serviceAccount.GetName())
	} else {
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.MintServiceAccountName,
				Namespace: constants.MintServiceAccountNamespace,
			},
		}

		// Create the service account
		apiContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := r.Create(apiContext, serviceAccount); err != nil {
			return ctrl.Result{}, err
		}
	}

	// create service account token
	token, err := r.mintServiceAccountToken(ctx, serviceAccount)
	if err != nil {
		r.log.Error(err, "failed to create service account token")
		return ctrl.Result{}, err
	}
	r.log.V(1).Info("Refresh token", "token", token)

	//copy token to secret
	if err := r.updateOrCreateCloudWatchSecret(ctx, token); err != nil {
		r.log.Error(err, "failed to create secret")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.TokenRefreshDuration}, nil
}

func (r *ServiceAccountReconciler) mintServiceAccountToken(
	ctx context.Context,
	serviceAccount *corev1.ServiceAccount,
) (refreshToken string, err error) {

	treq := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences: []string{"openshift"},
		},
	}

	// Create the service account token
	apiContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := r.ClientSet.CoreV1().ServiceAccounts(serviceAccount.GetNamespace()).CreateToken(apiContext, serviceAccount.GetName(), treq, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			r.log.V(1).Info("Token already exists", "Name", token.GetName())
		} else {
			r.log.Error(err, "failed to create token")
			return "", fmt.Errorf("failed to create token: %w", err)
		}
	}

	return token.Status.Token, nil
}

// checkAuditLogEnabled reads the secret/cloudwatch-credentials, if it contains the role arn value format
// we think the audit log forwarder is enabled
func (r *ServiceAccountReconciler) checkAuditLogEnabled(ctx context.Context) (bool, error) {

	sec := &corev1.Secret{}

	err := r.MCClient.Get(ctx, types.NamespacedName{Name: constants.CloudWatchSecretName, Namespace: r.HCPNamespace}, sec)
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	credStr := string(sec.Data["credentials"])

	if strings.Contains(credStr, "arn:aws:iam::") {
		return true, nil
	}

	return false, nil
}

func (r *ServiceAccountReconciler) updateOrCreateCloudWatchSecret(
	ctx context.Context,
	token string,
) error {

	r.log.V(1).Info("Creating secrets with token")

	var ocmCloudwatchSecret, cloCloudwatchSecret = &corev1.Secret{}, &corev1.Secret{}

	if err := r.MCClient.Get(ctx, types.NamespacedName{Name: constants.CloudWatchSecretName, Namespace: r.HCPNamespace}, ocmCloudwatchSecret); err != nil {
		r.log.Error(err, "failed to get cloud watch secret")
		return err
	}
	cloSecretExists := false
	err := r.MCClient.Get(ctx, types.NamespacedName{Name: constants.CollectorCloudWatchSecretName, Namespace: r.HCPNamespace}, cloCloudwatchSecret)
	if err != nil && errors.IsNotFound(err) {
		cloSecretExists = false
	} else if err == nil {
		cloSecretExists = true
	} else {
		return err
	}

	if cloSecretExists {
		cloCloudwatchSecret.Data["credentials"] = ocmCloudwatchSecret.Data["credentials"]
		cloCloudwatchSecret.Data["token"] = []byte(token)
		if err := r.MCClient.Update(ctx, cloCloudwatchSecret); err != nil {
			r.log.Error(err, "failed to update secret")
			return err
		}
	} else {
		cloCloudwatchSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.CollectorCloudWatchSecretName,
				Namespace: r.HCPNamespace,
			},
			Data: map[string][]byte{
				"config":      ocmCloudwatchSecret.Data["config"],
				"credentials": ocmCloudwatchSecret.Data["credentials"],
				"token":       []byte(token),
			},
		}

		if err := r.MCClient.Create(ctx, cloCloudwatchSecret); err != nil {
			r.log.Error(err, "failed to create secret")
			return err
		}
	}

	return nil
}
