package main

// Global configuration

const (
	// keys
	keyRedisHost                = "REDIS_HOST"
	keyRedisPort                = "REDIS_PORT"
	keyLastUpdatePrefixTemplate = "LAST_UPDATE_PREFIX_TEMPLATE"
	keyMetricsPrefixTemplate    = "METRICS_PREFIX_TEMPLATE"

	// default values
	defaultRedisHost                = "0.0.0.0"
	defaultRedisPort                = "6379"
	defaultLastUpdatePrefixTemplate = "deploymentid:last_action"
	defaultMetricsPrefixTemplate    = "deploymentid:minio-metrics"
)

// Local configuration

const (
	// keys
	keyDeploymentId       = "deploymentid"
	keyIsActiveTtlSeconds = "isActiveTtlSeconds"
	keyScaleMetricName    = "scaleMetricName"
	keyScalePeriodSeconds = "scalePeriodSeconds"
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

// Scale Metric Names

const (
	keyScaleMetricBytesIn         = "bytes_in"
	keyScaleMetricBytesOut        = "bytes_out"
	keyScaleMetricNumRequestsIn   = "num_requests_in"
	keyScaleMetricNumRequestsOut  = "num_requests_out"
	keyScaleMetricNumRequestsMisc = "num_requests_misc"

	// aggregates
	keyScaleMetricBytesTotal       = "bytes_total"
	keyScaleMetricNumRequestsInOut = "num_requests_in_out"
	keyScaleMetricNumRequestsTotal = "num_requests_total"
)
