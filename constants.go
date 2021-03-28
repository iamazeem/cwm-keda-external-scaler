package main

// Global configuration

const (
	// keys
	keyRedisHost                = "REDIS_HOST"
	keyRedisPort                = "REDIS_PORT"
	keyLastUpdatePrefixTemplate = "LAST_UPDATE_PREFIX_TEMPLATE"
	keyMetricsPrefixTemplate    = "METRICS_PREFIX_TEMPLATE"
	keyKubeConfigPath           = "KUBECONFIG"

	// default values
	defaultRedisHost                = "0.0.0.0"
	defaultRedisPort                = "6379"
	defaultLastUpdatePrefixTemplate = "deploymentid:last_action"
	defaultMetricsPrefixTemplate    = "deploymentid:minio-metrics"
	defaultKubeConfigPath           = ""
)

// Local configuration

const (
	// keys
	keyDeploymentId       = "deploymentid"
	keyIsActiveTtlSeconds = "isActiveTtlSeconds"
	keyScaleMetricName    = "scaleMetricName"
	keyScalePeriodSeconds = "scalePeriodSeconds"
	keyNamespaceName      = "namespaceName"
	keyDeploymentNames    = "deploymentNames"
	keyTargetValue        = "targetValue"

	// default values
	defaultDeploymentId       = "deploymentid"
	defaultIsActiveTtlSeconds = "600"
	defualtScaleMetricName    = "bytes_out"
	defaultScalePeriodSeconds = "600"
	defaultNamespaceName      = "default"
	defaultDeploymentNames    = ""
	defaultTargetValue        = "10"
)
