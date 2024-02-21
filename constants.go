package main

// Global configuration (environment variables)

const (
	// keys
	keyLogLevel         = "LOG_LEVEL"
	keyRedisHost        = "CWM_REDIS_HOST" // Added CWM_ prefix for REDIS_HOST and REDIS_PORT
	keyRedisPort        = "CWM_REDIS_PORT" // See: https://github.com/docker-library/redis/issues/53
	keyRedisDb          = "CWM_REDIS_DB"
	keyLastUpdatePrefix = "LAST_UPDATE_PREFIX"
	keyMetricsPrefix    = "METRICS_PREFIX"

	// default values
	defaultLogLevel         = "info"
	defaultRedisHost        = "localhost"
	defaultRedisPort        = "6379"
	defaultRedisDb          = "0"
	defaultLastUpdatePrefix = "deploymentid:last_action"
	defaultMetricsPrefix    = "deploymentid:minio-metrics"
)

// Local configuration (ScaledObject metadata)

const (
	// keys
	keyDeploymentId       = "deploymentid"
	keyIsActiveTtlSeconds = "isActiveTtlSeconds"
	keyScaleMetricName    = "scaleMetricName"
	keyScalePeriodSeconds = "scalePeriodSeconds"
	keyTargetValue        = "targetValue"

	// default values
	defaultDeploymentId       = "minio"
	defaultIsActiveTtlSeconds = "600"
	defaultScaleMetricName    = "bytes_out"
	defaultScalePeriodSeconds = "600"
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
