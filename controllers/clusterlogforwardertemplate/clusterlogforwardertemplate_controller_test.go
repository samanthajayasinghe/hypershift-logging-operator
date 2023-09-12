// /*
// Copyright 2023.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
package clusterlogforwardertemplate

//
//import (
//	"context"
//	"reflect"
//	"testing"
//
//	"github.com/go-logr/logr/testr"
//	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
//
//	hlov1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
//
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//
//	configv1 "github.com/openshift/api/config/v1"
//	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
//	corev1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/runtime"
//	"sigs.k8s.io/controller-runtime/pkg/client/fake"
//)
//
//func TestCreateClusterLogForwarder(t *testing.T) {
//	const namespace = "test-ns"
//
//	tests := []struct {
//		name                        string
//		clusterLogForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate
//		expectedClusterLogForwarder *loggingv1.ClusterLogForwarder
//		expectErr                   bool
//	}{
//		{
//			name: "should create new ClusterLogForwarder in given namespace",
//			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "instance",
//					Namespace: namespace,
//				},
//				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{
//
//					Template: loggingv1.ClusterLogForwarderSpec{
//						ServiceAccountName: "test-sa",
//					},
//				},
//			},
//			expectedClusterLogForwarder: &loggingv1.ClusterLogForwarder{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "instance",
//					Namespace: namespace,
//				},
//			},
//			expectErr: false,
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			c := NewTestMock(t).Client
//			r := &ClusterLogForwarderTemplateReconciler{
//				Client: c,
//				Scheme: c.Scheme(),
//				log:    testr.New(t),
//			}
//
//			err := r.Create(context.TODO(), test.clusterLogForwarderTemplate)
//			if err != nil {
//				if !test.expectErr {
//					t.Errorf("expected no err, got %v", err)
//				}
//				return
//			}
//
//			if test.expectErr {
//				t.Error("expected err, got nil")
//			}
//
//			if !test.expectErr {
//				clusterLogForwarder := new(loggingv1.ClusterLogForwarder)
//				if err := r.Get(context.TODO(), client.ObjectKey{
//					Namespace: namespace,
//					Name:      "instance",
//				}, clusterLogForwarder); err != nil {
//					t.Fatalf("unexpected err: %v", err)
//				}
//				if clusterLogForwarder.ObjectMeta.Namespace != test.expectedClusterLogForwarder.ObjectMeta.Namespace {
//					t.Errorf("mismatched Namespace, expected %v, got %v", clusterLogForwarder.ObjectMeta.Namespace, test.expectedClusterLogForwarder.ObjectMeta.Namespace)
//				}
//				if clusterLogForwarder.ObjectMeta.Name != test.expectedClusterLogForwarder.ObjectMeta.Name {
//					t.Errorf("mismatched Name, expected %v, got %v", clusterLogForwarder.ObjectMeta.Name, test.expectedClusterLogForwarder.ObjectMeta.Name)
//				}
//			}
//		})
//	}
//}
//
//func TestUpdateClusterLogForwarder(t *testing.T) {
//	const namespace = "test-ns"
//
//	tests := []struct {
//		name                                string
//		clusterLogForwarderTemplate         *hlov1alpha1.ClusterLogForwarderTemplate
//		modifiedClusterLogForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate
//		expectedClusterLogForwarder         *loggingv1.ClusterLogForwarder
//		expectErr                           bool
//	}{
//		{
//			name: "should update ClusterLogForwarder ",
//			modifiedClusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "instance",
//					Namespace: namespace,
//				},
//				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{
//					Template: loggingv1.ClusterLogForwarderSpec{
//						Inputs: []loggingv1.InputSpec{
//							{
//								Name:  "test-input-1",
//								Audit: &loggingv1.Audit{},
//							},
//						},
//					},
//				},
//			},
//			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "instance",
//					Namespace: namespace,
//				},
//				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{
//					Template: loggingv1.ClusterLogForwarderSpec{
//						Inputs: []loggingv1.InputSpec{
//							{
//								Name:  "test-input-2",
//								Audit: &loggingv1.Audit{},
//							},
//						}},
//				},
//			},
//			expectedClusterLogForwarder: &loggingv1.ClusterLogForwarder{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "instance",
//					Namespace: namespace,
//				},
//			},
//			expectErr: false,
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			c := NewTestMock(t).Client
//			r := &ClusterLogForwarderTemplateReconciler{
//				Client: c,
//				Scheme: c.Scheme(),
//				log:    testr.New(t),
//			}
//
//			// create it first
//			err := r.Create(context.TODO(), test.clusterLogForwarderTemplate)
//			if err != nil {
//				t.Errorf("creating log forwarder template: err %v", err)
//				return
//			}
//
//			newCLF := new(loggingv1.ClusterLogForwarder)
//			err = r.Get(context.TODO(), client.ObjectKey{Name: "instance", Namespace: namespace}, newCLF)
//			if err != nil {
//				t.Errorf("error getting log forwardewr: %v", err)
//				return
//			}
//
//			// update the template
//			err = r.Update(context.TODO(), test.modifiedClusterLogForwarderTemplate)
//			if err != nil {
//				t.Errorf("update template with err: %v", err)
//				return
//			}
//
//			actualClusterLogForwarder := new(loggingv1.ClusterLogForwarder)
//			if err := r.Get(context.TODO(), client.ObjectKey{
//				Namespace: namespace,
//				Name:      "instance",
//			}, actualClusterLogForwarder); err != nil {
//				t.Fatalf("unexpected err: %v", err)
//			}
//
//			if test.expectErr {
//				t.Error("expected err, got nil")
//			}
//
//			if !test.expectErr {
//				if !reflect.DeepEqual(actualClusterLogForwarder.Spec, test.modifiedClusterLogForwarderTemplate.Spec.Template) {
//					t.Errorf("mismatched spec, expected %v, got %v", actualClusterLogForwarder.Spec, test.modifiedClusterLogForwarderTemplate.Spec.Template)
//				}
//
//			}
//		})
//	}
//}
//
//func TestFilterHostedControlPlanes(t *testing.T) {
//	const namespace = "test-ns"
//	tests := []struct {
//		name                        string
//		clusterLogForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate
//		mockHCPs                    []client.Object //*hyperv1beta1.HostedControlPlane
//		expectedCount               int
//		expectErr                   bool
//	}{
//		{
//			name: "empty hosted clusters",
//			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "sample",
//					Namespace: namespace,
//				},
//				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{
//
//					Template: loggingv1.ClusterLogForwarderSpec{
//						ServiceAccountName: "test-valid",
//					},
//				},
//			},
//			mockHCPs:      nil,
//			expectedCount: 0,
//			expectErr:     false,
//		},
//		{
//			name: "multi hosted clusters",
//			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "sample",
//					Namespace: namespace,
//				},
//				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{
//
//					Template: loggingv1.ClusterLogForwarderSpec{
//						ServiceAccountName: "test-valid",
//					},
//				},
//			},
//			mockHCPs: []client.Object{
//				&hyperv1beta1.HostedControlPlane{
//					ObjectMeta: metav1.ObjectMeta{
//						Name:      "name1",
//						Namespace: "namespace1",
//					},
//				},
//				&hyperv1beta1.HostedControlPlane{
//					ObjectMeta: metav1.ObjectMeta{
//						Name:      "name2",
//						Namespace: "namespace2",
//					},
//				},
//				&hyperv1beta1.HostedControlPlane{
//					ObjectMeta: metav1.ObjectMeta{
//						Name:      "name3",
//						Namespace: "namespace3",
//					},
//				},
//			},
//			expectedCount: 3,
//			expectErr:     false,
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			client := NewTestMock(t).Client
//			if test.mockHCPs != nil {
//				client = NewTestMock(t, test.mockHCPs...).Client
//			}
//			r := &ClusterLogForwarderTemplateReconciler{
//				Client: client,
//				Scheme: client.Scheme(),
//				log:    testr.New(t),
//			}
//
//			actual, err := r.GetHostedControlPlanes(context.TODO())
//			if err != nil {
//				if !test.expectErr {
//					t.Errorf("expected no err, got %s", err)
//				}
//			} else {
//				if test.expectedCount != len(actual) {
//					t.Errorf("expected %v hostedcontrolplanes, got %v", test.expectedCount, len(actual))
//				}
//			}
//		})
//	}
//}
//
//func TestValidateLogForwarderInHostedControlPlanes(t *testing.T) {
//	const namespace = "test-ns"
//	tests := []struct {
//		name                        string
//		clusterLogForwarderTemplate *hlov1alpha1.ClusterLogForwarderTemplate
//		mockHCPs                    []hyperv1beta1.HostedControlPlane
//		expectErr                   bool
//	}{
//		{
//			name: "multi hosted clusters",
//			clusterLogForwarderTemplate: &hlov1alpha1.ClusterLogForwarderTemplate{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "sample",
//					Namespace: namespace,
//				},
//				Spec: hlov1alpha1.ClusterLogForwarderTemplateSpec{
//
//					Template: loggingv1.ClusterLogForwarderSpec{
//						ServiceAccountName: "test-valid",
//					},
//				},
//			},
//			mockHCPs: []hyperv1beta1.HostedControlPlane{
//				{
//					ObjectMeta: metav1.ObjectMeta{
//						Name:      "name1",
//						Namespace: "namespace1",
//					},
//				},
//				{
//					ObjectMeta: metav1.ObjectMeta{
//						Name:      "name2",
//						Namespace: "namespace2",
//					},
//				},
//				{
//					ObjectMeta: metav1.ObjectMeta{
//						Name:      "name3",
//						Namespace: "namespace3",
//					},
//				},
//			},
//			expectErr: false,
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			c := NewTestMock(t).Client
//
//			r := &ClusterLogForwarderTemplateReconciler{
//				Client: c,
//				Scheme: c.Scheme(),
//				log:    testr.New(t),
//			}
//
//			//check all HCP NS
//			for _, hcpNS := range test.mockHCPs {
//				nsClusterLogForwarder := new(loggingv1.ClusterLogForwarder)
//				if err := r.Get(context.TODO(), client.ObjectKey{
//					Namespace: hcpNS.ObjectMeta.Namespace,
//					Name:      test.clusterLogForwarderTemplate.Name,
//				}, nsClusterLogForwarder); err != nil {
//					t.Fatalf("unexpected err: %v", err)
//				}
//
//				if !reflect.DeepEqual(nsClusterLogForwarder.Spec, test.clusterLogForwarderTemplate.Spec.Template) {
//					t.Errorf("mismatched spec, expected %v, got %v", nsClusterLogForwarder.Spec, test.clusterLogForwarderTemplate.Spec.Template)
//				}
//			}
//		})
//	}
//}
//
//type MockKubeClient struct {
//	Client client.Client
//}
//
//func NewTestMock(t *testing.T, objs ...client.Object) *MockKubeClient {
//	mock, err := NewMock(objs...)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	return mock
//}
//
//func NewMock(obs ...client.Object) (*MockKubeClient, error) {
//	s := runtime.NewScheme()
//	if err := corev1.AddToScheme(s); err != nil {
//		return nil, err
//	}
//
//	if err := configv1.Install(s); err != nil {
//		return nil, err
//	}
//
//	if err := hyperv1beta1.AddToScheme(s); err != nil {
//		return nil, err
//	}
//
//	if err := loggingv1.AddToScheme(s); err != nil {
//		return nil, err
//	}
//
//	if err := hlov1alpha1.AddToScheme(s); err != nil {
//		return nil, err
//	}
//
//	return &MockKubeClient{
//		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(obs...).Build(),
//	}, nil
//}
