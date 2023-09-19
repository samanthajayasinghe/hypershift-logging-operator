/*
	 Copyright 2023.

	 Licensed under the Apache License, Version 2.0 (the "License");
	 you may not use this file except in compliance with the License.
	 You may obtain a copy of the License at

		http:www.apache.org/licenses/LICENSE-2.0

	 Unless required by applicable law or agreed to in writing, software
	 distributed under the License is distributed on an "AS IS" BASIS,
	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	 See the License for the specific language governing permissions and
	 limitations under the License.
*/
package hostedcluster

import (
	"context"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"

	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1 "github.com/openshift/api/config/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func TestGetHostedControlPlanes(t *testing.T) {
	const namespace = "test-ns"
	var mockHcpList = []client.Object{
		&hyperv1beta1.HostedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name1",
				Namespace: "namespace1",
			},
			Status: hyperv1beta1.HostedControlPlaneStatus{
				Ready: true,
			},
		},
		&hyperv1beta1.HostedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name2",
				Namespace: "namespace2",
			},
			Status: hyperv1beta1.HostedControlPlaneStatus{
				Ready: true,
			},
		},
		&hyperv1beta1.HostedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name3",
				Namespace: "namespace3",
			},
			Status: hyperv1beta1.HostedControlPlaneStatus{
				Ready: false,
			},
		},
	}
	tests := []struct {
		name          string
		mockHCPs      []client.Object
		expectedCount int
		onlyActive    bool
	}{
		{
			name:          "All HCP",
			mockHCPs:      mockHcpList,
			expectedCount: 3,
			onlyActive:    false,
		},
		{
			name:          "Only active HCP",
			mockHCPs:      mockHcpList,
			expectedCount: 2,
			onlyActive:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			c := NewTestMock(t, test.mockHCPs...).Client

			hcpList, _ := GetHostedControlPlanes(c, context.TODO(), test.onlyActive)

			if len(hcpList) != test.expectedCount {
				t.Errorf("mismatched Count, expected %v, got %v", test.expectedCount, len(hcpList))
			}
		})
	}
}

type MockKubeClient struct {
	Client client.Client
}

func NewTestMock(t *testing.T, objs ...client.Object) *MockKubeClient {
	mock, err := NewMock(objs...)
	if err != nil {
		t.Fatal(err)
	}

	return mock
}

func NewMock(obs ...client.Object) (*MockKubeClient, error) {
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := configv1.Install(s); err != nil {
		return nil, err
	}

	if err := hyperv1beta1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := loggingv1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := hlov1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	return &MockKubeClient{
		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(obs...).Build(),
	}, nil
}
