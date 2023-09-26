package hostedcluster

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KubeConfigSecret = "service-network-admin-kubeconfig"
	KubeAPISvcName   = "kube-apiserver"
	KubeDnsSuffix    = "svc.cluster.local"
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

	hcList := new(hyperv1beta1.HostedClusterList)
	if err := c.List(ctx, hcList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	if onlyActive {
		var filterList []hyperv1beta1.HostedCluster

		for _, hc := range hcList.Items {
			ready := false
			progress := false
			for i := range hc.Status.Conditions {
				c := hc.Status.Conditions[i]

				if c.Type == "Available" && c.Status == v1.ConditionTrue {
					ready = true
				}
			}

			for j := range hc.Status.Version.History {
				history := hc.Status.Version.History[j]
				if history.State == "Completed" {
					progress = true
				}
			}

			if ready && progress {
				filterList = append(filterList, hc)
			}
		}
		return filterList, nil
	}

	return hcList.Items, nil
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
