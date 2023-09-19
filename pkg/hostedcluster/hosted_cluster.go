package hostedcluster

import (
	"context"

	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetHostedControlPlanes returns a list of all hostedcontrolplane based search criteria
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
