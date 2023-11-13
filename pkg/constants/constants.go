package constants

import "time"

const (
	ManagedLoggingFinalizer       = "logging.managed.openshift.io"
	HLFWatchedNamespace           = "openshift-logging"
	OperatorNamespace             = "openshift-hypershift-logging-operator"
	MintServiceAccountNamespace   = "openshift-config-managed"
	MintServiceAccountName        = "cloudwatch-audit-exporter"
	TokenRefreshDuration          = time.Minute * 30
	CloudWatchSecretName          = "cloudwatch-credentials"
	CollectorCloudWatchSecretName = "collector-cloudwatch-credentials"
)
