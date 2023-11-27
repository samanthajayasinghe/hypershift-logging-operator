package clusterlogforwarder

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
)

var (
	InputHTTPServerName = "input-httpserver"
	InputHTTPServerSpec = loggingv1.InputSpec{
		Name: InputHTTPServerName,
		Receiver: &loggingv1.ReceiverSpec{
			HTTP: &loggingv1.HTTPReceiver{
				Format: "kubeAPIAudit",
				Port:   8443,
			},
		},
	}
)

type ClusterLogForwarderBuilder struct {
	Clf  *loggingv1.ClusterLogForwarder
	Hlf  *v1alpha1.HyperShiftLogForwarder
	Clft *v1alpha1.ClusterLogForwarderTemplate
}

// BuildInputsFromTemplate builds the input array from the template
func (b *ClusterLogForwarderBuilder) BuildInputsFromTemplate() *ClusterLogForwarderBuilder {

	if len(b.Clf.Spec.Inputs) < 1 {
		b.Clf.Spec.Inputs = append(b.Clf.Spec.Inputs, InputHTTPServerSpec)
	}

	//if len(template.Spec.Template.Inputs) > 0 {
	//	for _, input := range template.Spec.Template.Inputs {
	//		clf.Spec.Inputs = append(clf.Spec.Inputs, input)
	//	}
	//}

	return b
}

// BuildOutputsFromTemplate builds the output array from the template
func (b *ClusterLogForwarderBuilder) BuildOutputsFromTemplate() *ClusterLogForwarderBuilder {

	if len(b.Clft.Spec.Template.Outputs) > 0 {
		for _, output := range b.Clft.Spec.Template.Outputs {
			b.Clf.Spec.Outputs = append(b.Clf.Spec.Outputs, output)
		}
	}

	return b
}

// BuildPipelinesFromTemplate builds the pipeline array from the template
func (b *ClusterLogForwarderBuilder) BuildPipelinesFromTemplate() *ClusterLogForwarderBuilder {

	if len(b.Clft.Spec.Template.Pipelines) > 0 {
		for _, ppl := range b.Clft.Spec.Template.Pipelines {
			b.Clf.Spec.Pipelines = append(b.Clf.Spec.Pipelines, ppl)
		}
	}

	return b
}

// BuildFiltersFromTemplate builds the filter array from the template
func (b *ClusterLogForwarderBuilder) BuildFiltersFromTemplate() *ClusterLogForwarderBuilder {

	if len(b.Clft.Spec.Template.Filters) > 0 {
		for _, f := range b.Clft.Spec.Template.Filters {
			b.Clf.Spec.Filters = append(b.Clf.Spec.Filters, f)
		}
	}

	return b
}

// BuildServiceAccount builds the service account for CLF
func (b *ClusterLogForwarderBuilder) BuildServiceAccount() *ClusterLogForwarderBuilder {
	if b.Clf.Spec.ServiceAccountName == "" {
		b.Clf.Spec.ServiceAccountName = "default"
	}

	return b
}

// BuildInputsFromHLF builds the CLF inputs from the HLF
func (b *ClusterLogForwarderBuilder) BuildInputsFromHLF() *ClusterLogForwarderBuilder {

	if len(b.Clf.Spec.Inputs) < 1 {
		b.Clf.Spec.Inputs = append(b.Clf.Spec.Inputs, InputHTTPServerSpec)
	}

	return b
}

// BuildOutputsFromHLF builds the CLF outputs from the HLF
func (b *ClusterLogForwarderBuilder) BuildOutputsFromHLF() *ClusterLogForwarderBuilder {
	if len(b.Hlf.Spec.Outputs) > 0 {
		for _, output := range b.Hlf.Spec.Outputs {
			b.Clf.Spec.Outputs = append(b.Clf.Spec.Outputs, output)
		}
	}

	return b
}

// BuildPipelinesFromHLF builds the CLF pipelines from the HLF
func (b *ClusterLogForwarderBuilder) BuildPipelinesFromHLF(labels map[string]string) *ClusterLogForwarderBuilder {
	if len(b.Hlf.Spec.Pipelines) > 0 {
		for index, ppl := range b.Hlf.Spec.Pipelines {
			b.Clf.Spec.Pipelines = append(b.Clf.Spec.Pipelines, ppl)
			// Add labels to pipeline
			if len(labels) > 0 {
				b.Clf.Spec.Pipelines[index].Labels = labels
			}
		}
	}

	return b
}

// BuildFiltersFromHLF builds the CLF filters from the HLF
func (b *ClusterLogForwarderBuilder) BuildFiltersFromHLF() *ClusterLogForwarderBuilder {
	if len(b.Hlf.Spec.Filters) > 0 {
		for _, f := range b.Hlf.Spec.Filters {
			b.Clf.Spec.Filters = append(b.Clf.Spec.Filters, f)
		}
	}
	return b
}
