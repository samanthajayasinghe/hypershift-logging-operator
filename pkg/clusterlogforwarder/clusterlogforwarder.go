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
func BuildInputsFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(clf.Spec.Inputs) < 1 {
		clf.Spec.Inputs = append(clf.Spec.Inputs, InputHTTPServerSpec)
	}

	//if len(hlf.Spec.Inputs) > 0 {
	//	for _, input := range hlf.Spec.Inputs {
	//		if !strings.Contains(input.Name, constants.CustomerManagedRuleNamePrefix) {
	//			input.Name = constants.CustomerManagedRuleNamePrefix + "-" + input.Name
	//		}
	//		clf.Spec.Inputs = append(clf.Spec.Inputs, input)
	//	}
	//}
	return clf
}

// BuildOutputsFromHLF builds the CLF outputs from the HLF
func BuildOutputsFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	if len(hlf.Spec.Outputs) > 0 {
		for _, output := range hlf.Spec.Outputs {
			clf.Spec.Outputs = append(clf.Spec.Outputs, output)
		}
	}

	return clf
}

// BuildPipelinesFromHLF builds the CLF pipelines from the HLF
func BuildPipelinesFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	if len(hlf.Spec.Pipelines) > 0 {
		for _, ppl := range hlf.Spec.Pipelines {
			clf.Spec.Pipelines = append(clf.Spec.Pipelines, ppl)
		}
	}

	return clf
}

// BuildFiltersFromHLF builds the CLF filters from the HLF
func BuildFiltersFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	if len(hlf.Spec.Filters) > 0 {
		for _, f := range hlf.Spec.Filters {
			clf.Spec.Filters = append(clf.Spec.Filters, f)
		}
	}
	return clf
}
