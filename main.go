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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-logr/logr"
	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/controllers/clusterlogforwardertemplate"
	"github.com/openshift/hypershift-logging-operator/controllers/hypershiftlogforwarder"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	clusters = map[string]cluster.Cluster{}
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(hlov1alpha1.AddToScheme(scheme))

	//Add hypershift.openshift.io for the hostedcontrolplanes CR
	utilruntime.Must(hyperv1beta1.AddToScheme(scheme))

	//Add logging.openshift.io for the ClusterLogForwarder CR
	utilruntime.Must(loggingv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme

	//temp setup sub-A cluster
	restConfigSubA, err := createGuestKubeconfig(context.Background(), "ocm-stg-hs-two", setupLog)
	if err != nil {
		setupLog.Error(err, "getting guest cluster kubeconfig")
	}
	subAcluster, err := cluster.New(restConfigSubA)
	if err != nil {
		setupLog.Error(err, "creating guest cluster kubeconfig")
	}
	clusters["sub"] = subAcluster

	// temp setup sub-b cluster
	restConfigSubB, err := createGuestKubeconfig(context.Background(), "ocm-stg-hs-one", setupLog)
	if err != nil {
		setupLog.Error(err, "getting guest cluster kubeconfig")
	}
	subBcluster, err := cluster.New(restConfigSubB)
	if err != nil {
		setupLog.Error(err, "creating guest cluster kubeconfig")
	}
	clusters["sub-b"] = subBcluster
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0b68d538.logging.managed.openshift.io",
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
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	// set up clusters
	setupLog.Info("set up clusters ")
	for name, cluster := range clusters {
		clusterScheme := cluster.GetScheme()
		utilruntime.Must(hlov1alpha1.AddToScheme(clusterScheme))
		clusters[name] = cluster
		mgr.Add(cluster)
	}
	_, err = hypershiftlogforwarder.NewHyperShiftLogForwarderReconciler(mgr, clusters)

	setupLog.Info("Registering Components.")

	if err = (&clusterlogforwardertemplate.ClusterLogForwarderTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create ClusterLogForwarderTemplate controller", "controller", "ClusterLogForwarderTemplate")
		os.Exit(1)
	}

	/*if err = (&hypershiftlogforwarder.HyperShiftLogForwarderReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create HyperShiftLogForwarderReconciler controller", "controller", "HyperShiftLogForwarderReconciler")
		os.Exit(1)
	}*/

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}

func GetHostedClusters() ([]hyperv1beta1.HostedControlPlane, error) {
	cl, err := client.New(config.GetConfigOrDie(), client.Options{})

	if err != nil {
		setupLog.Error(err, "failed to create client")
		os.Exit(1)
	}

	hcpList := new(hyperv1beta1.HostedControlPlaneList)
	if err := cl.List(context.Background(), hcpList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	return hcpList.Items, nil
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

// oc get secret 267hpof1a3bpgjildo6locrmqhmd3ml3-kubeconfig -n ocm-staging-267hpof1a3bpgjildo6locrmqhmd3ml3-hcpeast2latest --as backplane-cluster-admin | grep kubeconfig
