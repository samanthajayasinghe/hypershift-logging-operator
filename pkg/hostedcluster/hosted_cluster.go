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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func CreateGuestKubeconfig(
	c client.Client,
	cpNamespace string,
	log logr.Logger,
) (*rest.Config, error) {

	localhostKubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "localhost-kubeconfig",
			Namespace: cpNamespace,
		},
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(localhostKubeconfigSecret), localhostKubeconfigSecret); err != nil {
		return nil, fmt.Errorf("failed to get hostedcluster localhost kubeconfig: %w", err)
	}

	localhostKubeRoute := &ocroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: cpNamespace,
		},
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(localhostKubeRoute), localhostKubeRoute); err != nil {
		return nil, fmt.Errorf("failed to get kube-apiserver-internal route: %w", err)
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

	log.Info("connecting API server  cluster", "api-endpoint", localhostKubeRoute.Spec.Host)

	for k := range localhostKubeconfig.Clusters {
		localhostKubeconfig.Clusters[k].Server = fmt.Sprintf("https://%s:6443", localhostKubeRoute.Spec.Host)
	}

	localhostKubeconfigYaml, err := clientcmd.Write(*localhostKubeconfig)
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(localhostKubeconfigYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize localhost kubeconfig: %w", err)
	}

	return restConfig, nil
}
