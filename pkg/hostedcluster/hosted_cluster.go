package hostedcluster

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	ocroutev1 "github.com/openshift/api/route/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KubeConfigSecret                    = "service-network-admin-kubeconfig"
	KubeAPISvcName                      = "kube-apiserver"
	KubeDnsSuffix                       = "svc.cluster.local"
	HostedClusterAvailableCondition     = "Available"
	HostedClusterVersionCompletedStatus = "Completed"
)

// GetHostedControlPlanes returns a list of all hostedcontrolplane based on search criteria
func GetHostedControlPlanes(
	c client.Client,
	ctx context.Context,
	onlyActive bool,
) ([]hyperv1beta1.HostedControlPlane, error) {

	hcpList := new(hyperv1beta1.HostedControlPlaneList)
	if err := c.List(ctx, hcpList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	if onlyActive {
		var filterList []hyperv1beta1.HostedControlPlane
		for _, hcp := range hcpList.Items {

			if hcp.Status.Ready {
				filterList = append(filterList, hcp)
			}
		}
		return filterList, nil
	}

	return hcpList.Items, nil
}

// GetHostedClusters returns a list of all HostedClusters based on search criteria
func GetHostedClusters(
	c client.Client,
	ctx context.Context,
	onlyActive bool,
	log logr.Logger,
) ([]hyperv1beta1.HostedCluster, error) {

	utilruntime.Must(hyperv1beta1.AddToScheme(c.Scheme()))
	utilruntime.Must(ocroutev1.AddToScheme(c.Scheme()))

	hcList := new(hyperv1beta1.HostedClusterList)
	if err := c.List(ctx, hcList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	if onlyActive {
		var filterList []hyperv1beta1.HostedCluster

		for _, hc := range hcList.Items {
			if IsReadyHostedCluster(hc) {
				filterList = append(filterList, hc)
			}
		}
		return filterList, nil
	}

	return hcList.Items, nil
}

// IsReadyHostedCluster returns true if hostedcuster is ready and Completed
func IsReadyHostedCluster(hostedCluster hyperv1beta1.HostedCluster) bool {
	ready := false

	for i := range hostedCluster.Status.Conditions {
		c := hostedCluster.Status.Conditions[i]

		if c.Type == HostedClusterAvailableCondition && c.Status == v1.ConditionTrue {
			ready = true
		}
	}

	/*
		progress := false
		if hostedCluster.Status.Version != nil {
			for j := range hostedCluster.Status.Version.History {
				history := hostedCluster.Status.Version.History[j]
				if history.State == HostedClusterVersionCompletedStatus {
					progress = true
				}
			}
		}*/

	if ready {
		//if ready && progress {
		return true
	}
	return false
}

// BuildGuestKubeConfig builds the kubeconfig for client to access the hosted cluster from the secrets in HCP namespace
func BuildGuestKubeConfig(
	c client.Client,
	hcpNamespace string,
	log logr.Logger,
) (*rest.Config, error) {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KubeConfigSecret,
			Namespace: hcpNamespace,
		},
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(secret), secret); err != nil {
		return nil, fmt.Errorf("failed to get hostedcluster admin kubeconfig: %w", err)
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
	kubeConfig, err := clientcmd.Load(secret.Data["kubeconfig"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig from the secret: %w", err)
	}
	if len(kubeConfig.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found in admin kubeconfig")
	}

	serverAPI := fmt.Sprintf("https://%s.%s.%s:6443", KubeAPISvcName, hcpNamespace, KubeDnsSuffix)
	log.Info("connecting API server  cluster", "api-endpoint", serverAPI)

	for k := range kubeConfig.Clusters {
		kubeConfig.Clusters[k].Server = serverAPI
	}

	kubeConfigYaml, err := clientcmd.Write(*kubeConfig)
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize kubeconfig: %w", err)
	}

	return restConfig, nil
}

// Validate kube config
func ValidateKubeConfig(c client.Client, hcpNamespace string) (bool, error) {

	//check the secrets
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KubeConfigSecret,
			Namespace: hcpNamespace,
		},
	}

	if err := c.Get(context.Background(), client.ObjectKeyFromObject(secret), secret); err != nil {
		return false, fmt.Errorf("failed to get hostedcluster admin kubeconfig: %w", err)
	}

	//check the kubeconfig
	kubeConfig, err := clientcmd.Load(secret.Data["kubeconfig"])
	if err != nil {
		return false, fmt.Errorf("failed to parse kubeconfig from the secret: %w", err)
	}
	if len(kubeConfig.Clusters) == 0 {
		return false, fmt.Errorf("no clusters found in admin kubeconfig")
	}

	return true, nil
}
