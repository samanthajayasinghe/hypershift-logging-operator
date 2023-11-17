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
				ReceiverPort: loggingv1.ReceiverPort{
					Name:       "httpserver",
					Port:       443,
					TargetPort: 8443,
				},
			},
		},
	}
)

type ClusterLogForwarderBuilder struct {
	Clf *loggingv1.ClusterLogForwarder
	Hlf *v1alpha1.HyperShiftLogForwarder
}

// BuildInputsFromTemplate builds the input array from the template
func BuildInputsFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(clf.Spec.Inputs) < 1 {
		clf.Spec.Inputs = append(clf.Spec.Inputs, InputHTTPServerSpec)
	}

	//if len(template.Spec.Template.Inputs) > 0 {
	//	for _, input := range template.Spec.Template.Inputs {
	//		clf.Spec.Inputs = append(clf.Spec.Inputs, input)
	//	}
	//}

	return clf
}

// BuildOutputsFromTemplate builds the output array from the template
func BuildOutputsFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(template.Spec.Template.Outputs) > 0 {
		for _, output := range template.Spec.Template.Outputs {
			clf.Spec.Outputs = append(clf.Spec.Outputs, output)
		}
	}

	return clf
}

// BuildPipelinesFromTemplate builds the pipeline array from the template
func BuildPipelinesFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(template.Spec.Template.Pipelines) > 0 {
		for _, ppl := range template.Spec.Template.Pipelines {
			clf.Spec.Pipelines = append(clf.Spec.Pipelines, ppl)
		}
	}

	return clf
}

// BuildFiltersFromTemplate builds the filter array from the template
func BuildFiltersFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(template.Spec.Template.Filters) > 0 {
		for _, f := range template.Spec.Template.Filters {
			clf.Spec.Filters = append(clf.Spec.Filters, f)
		}
	}

	return clf
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
