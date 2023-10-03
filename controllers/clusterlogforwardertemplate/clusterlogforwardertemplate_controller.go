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
	coreError "errors"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/controllers/hypershiftlogforwarder"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
)

var (
	currentBuilder *builder.Builder
	rctx           context.Context
	cancelFunc     context.CancelFunc
	subClusters    = map[string]hypershiftlogforwarder.HostedCluster{}
)

// ClusterLogForwarderTemplateReconciler reconciles a ClusterLogForwarderTemplate object
type ClusterLogForwarderTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
	Mgr    ctrl.Manager
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

	r.log.V(1).Info("testing", "namespace", req.Namespace)

	clusterLogForwarderTemplate := &hlov1alpha1.ClusterLogForwarderTemplate{}

	err := r.Get(ctx, req.NamespacedName, clusterLogForwarderTemplate)

	found := false
	if err != nil && errors.IsNotFound(err) {
		found = false
	} else if err == nil {
		found = true
	} else {
		return ctrl.Result{}, err
	}

	deletion := false
	// HyperShiftLogForwarder was deleted by CU
	if !clusterLogForwarderTemplate.DeletionTimestamp.IsZero() {
		deletion = true
	}
	r.log.V(1).Info("testing", "deletion", deletion, "found", found)
	if found {
		restConfigSubA, err := createGuestKubeconfig(context.Background(), "ocm-stg-hs-two", r.log)
		if err != nil {
			r.log.Error(err, "getting guest cluster kubeconfig")
		}
		subAcluster, err := cluster.New(restConfigSubA)
		if err != nil {
			r.log.Error(err, "creating guest cluster kubeconfig")
		}

		mgrSub, err := ctrl.NewManager(restConfigSubA, ctrl.Options{
			Scheme:                 r.Scheme,
			HealthProbeBindAddress: "",
			LeaderElection:         false,
			MetricsBindAddress:     "0",
			LeaderElectionID:       "0b68d5399.logging.managed.openshift.io",
			// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
			// when the Manager ends. This requires the binary to immediately end when the
			// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
			// speeds up voluntary leader transitions as the new leader don't have to wait
			// LeaseDuration time first.
			//
			// In the default scaffold provided, the program ends immediately after
			// the manager stops, so would be fine to enable this option. However,
			// if you are doing or is intended to do any operation such as perform cleanups
			// after the manager stops then its usage might be unsafe.
			// LeaderElectionReleaseOnCancel: true,
		})

		rctx = context.Background()
		rctx, cancelFunc = context.WithCancel(rctx)
		rhc := hypershiftlogforwarder.HyperShiftLogForwarderReconciler{
			Client:       mgrSub.GetClient(),
			Scheme:       r.Scheme,
			MCClient:     r.Client,
			HCPNamespace: "ocm-stg-hs-two",
			Log:          r.log,
		}

		hostedCluster := hypershiftlogforwarder.HostedCluster{
			Cluster:      subAcluster,
			HCPNamespace: "ocm-stg-hs-two",
			Context:      rctx,
			CancelFunc:   cancelFunc,
		}
		subClusters["abc"] = hostedCluster

		//Adding new hosted cluster to the manager
		clusterScheme := hostedCluster.Cluster.GetScheme()
		utilruntime.Must(hyperv1beta1.AddToScheme(clusterScheme))
		utilruntime.Must(v1alpha1.AddToScheme(clusterScheme))

		go func() {
			/*currentBuilder := ctrl.NewControllerManagedBy(mgrSub).
				Named("abc").
				For(&v1alpha1.HyperShiftLogForwarder{}).
				Watches(
					source.NewKindWithCache(&v1alpha1.HyperShiftLogForwarder{}, hostedCluster.Cluster.GetCache()),
					&handler.EnqueueRequestForObject{},
				)

			err = currentBuilder.Complete(&rhc)

			r.log.Info("starting sub manager")
			if err := mgrSub.Start(rctx); err != nil {
				r.log.Error(err, "problem running sub manager")
			}*/
			ctrl.NewControllerManagedBy(mgrSub).
				Named("abc").
				For(&hlov1alpha1.HyperShiftLogForwarder{}).
				Complete(&rhc)

			r.log.Info("starting sub manager")
			if err := mgrSub.Start(rctx); err != nil {
				r.log.Error(err, "problem running sub manager")
			}

		}()

		if err != nil {

			return ctrl.Result{}, err
		}

	} else {
		r.log.V(1).Info("testing", "found", found)
		subCluster, _ := subClusters["abc"]
		subCluster.CancelFunc()
		r.log.V(1).Info("finished context")
	}

	/*if req.NamespacedName.Name != consts.SingletonName {
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
	*/
	return ctrl.Result{}, nil
}

func eventPredicates() predicate.Predicate {
	return predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
}

func operation1(ctx context.Context) error {
	// Let's assume that this operation failed for some reason
	// We use time.Sleep to simulate a resource intensive operation
	time.Sleep(100 * time.Millisecond)
	return coreError.New("failed")
}

func operation2(ctx context.Context) {
	// We use a similar pattern to the HTTP server
	// that we saw in the earlier example
	select {

	case <-ctx.Done():
		fmt.Println("halted operation2")
	}
}

func createGuestKubeconfig(ctx context.Context, cpNamespace string, log logr.Logger) (*rest.Config, error) {

	c, err := client.New(config.GetConfigOrDie(), client.Options{})

	localhostKubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "localhost-kubeconfig",
			Namespace: cpNamespace,
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(localhostKubeconfigSecret), localhostKubeconfigSecret); err != nil {
		return nil, fmt.Errorf("failed to get hostedcluster localhost kubeconfig: %w", err)
	}
	kubeconfigFile, err := os.CreateTemp(os.TempDir(), "kubeconfig-")
	if err != nil {
		return nil, fmt.Errorf("failed to create tempfile for kubeconfig: %w", err)
	}
	defer func() {
		if err := kubeconfigFile.Sync(); err != nil {
			log.Error(err, "Failed to sync temporary kubeconfig file")
		}
		if err := kubeconfigFile.Close(); err != nil {
			log.Error(err, "Failed to close temporary kubeconfig file")
		}
	}()
	localhostKubeconfig, err := clientcmd.Load(localhostKubeconfigSecret.Data["kubeconfig"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse localhost kubeconfig: %w", err)
	}
	if len(localhostKubeconfig.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in localhost kubeconfig")
	}

	//for k := range localhostKubeconfig.Clusters {
	//	localhostKubeconfig.Clusters[k].Server = fmt.Sprintf("https://localhost:%d", localPort)
	//}
	localhostKubeconfigYaml, err := clientcmd.Write(*localhostKubeconfig)
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(localhostKubeconfigYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize localhost kubeconfig: %w", err)
	}

	return restConfig, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterLogForwarderTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hlov1alpha1.ClusterLogForwarderTemplate{}).
		WithEventFilter(eventPredicates()).
		Complete(r)
}
