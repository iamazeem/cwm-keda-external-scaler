package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type metric struct {
	name  string
	value int64
}

func getEnv(key, defaultValue string) string {
	key = strings.TrimSpace(key)
	log.Printf("getting environment variable: key = '%v', default = '%v'", key, defaultValue)
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		log.Printf("got: %v => %v", key, value)
		return value
	} else {
		log.Printf("environment variable '%v' not found! falling back to default value '%v'", key, defaultValue)
		return defaultValue
	}
}

func getLastUpdatPrefix(metadata map[string]string) string {
	lastUpdatePrefixTemplate := getEnv(keyLastUpdatePrefixTemplate, defaultLastUpdatePrefixTemplate)
	deploymentid := getValueFromScalerMetadata(metadata, keyDeploymentId, defaultDeploymentId)
	lastUpdatePrefix := strings.Replace(lastUpdatePrefixTemplate, keyDeploymentId, deploymentid, 1)
	if lastUpdatePrefix == "" {
		log.Printf("last update prefix is empty")
	}

	return lastUpdatePrefix
}

func getMetricsPrefix(metadata map[string]string) string {
	metricsPrefixTemplate := getEnv(keyMetricsPrefixTemplate, defaultMetricsPrefixTemplate)
	deploymentid := getValueFromScalerMetadata(metadata, keyDeploymentId, defaultDeploymentId)
	metricsPrefix := strings.Replace(metricsPrefixTemplate, keyDeploymentId, deploymentid, 1)
	if metricsPrefix == "" {
		log.Println("metrics prefix is empty")
	}

	return metricsPrefix
}

func getValueFromScalerMetadata(metadata map[string]string, key, defaultValue string) string {
	key = strings.TrimSpace(key)
	log.Printf("getting metadata value: key = '%v', default = '%v'", key, defaultValue)
	if value, exists := metadata[key]; exists {
		return strings.TrimSpace(value)
	} else {
		log.Printf("metadata value '%v' not found! falling back to default value '%v'", key, defaultValue)
		return defaultValue
	}
}

// IsActive utility functions

func getIsActiveTtlSeconds(metadata map[string]string) (int64, error) {
	isActiveTtlSecondsStr := getValueFromScalerMetadata(metadata, keyIsActiveTtlSeconds, defaultIsActiveTtlSeconds)
	if isActiveTtlSeconds, err := parseInt64(isActiveTtlSecondsStr); err != nil {
		return -1, err
	} else if isActiveTtlSeconds < 0 {
		return -1, status.Errorf(codes.InvalidArgument, "invalid value: %v => %v", keyIsActiveTtlSeconds, isActiveTtlSeconds)
	} else {
		return isActiveTtlSeconds, nil
	}
}

func getLastUpdateTime(metadata map[string]string) (time.Time, error) {
	keyLastUpdate := getLastUpdatPrefix(metadata)
	lastUpdateValue, isValidLastUpdateValue := getValueFromRedisServer(keyLastUpdate)
	if !isValidLastUpdateValue {
		return time.Time{}, status.Errorf(codes.Internal, "invalid value: %v => %v", keyLastUpdate, lastUpdateValue)
	}

	lastUpdateTime, err := time.Parse(time.RFC3339Nano, lastUpdateValue)
	if err != nil {
		return time.Time{}, status.Errorf(codes.Internal, "invalid value: %v => %v", keyLastUpdate, lastUpdateTime)
	}

	return lastUpdateTime, nil
}

func getScalePeriodSeconds(metadata map[string]string) (int64, error) {
	scalePeriodSecondsStr := getValueFromScalerMetadata(metadata, keyScalePeriodSeconds, defaultScalePeriodSeconds)
	if scalePeriodSeconds, err := parseInt64(scalePeriodSecondsStr); err != nil {
		return -1, err
	} else if scalePeriodSeconds < 0 {
		return -1, status.Errorf(codes.InvalidArgument, "invalid value: %v => %v", keyScalePeriodSeconds, scalePeriodSeconds)
	} else {
		return scalePeriodSeconds, nil
	}
}

// GetMetrics utility functions

func parseInt64(s string) (int64, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err != nil {
		return -1, status.Errorf(codes.InvalidArgument, "parsing failed: %v => %v [%v]", s, v, err.Error())
	} else {
		return v, nil
	}
}

func parseMetricValue(metricValueStr string) (int64, error) {
	if metricValue, err := parseInt64(metricValueStr); err != nil {
		return -1, err
	} else if metricValue < 0 {
		return -1, status.Errorf(codes.InvalidArgument, "invalid %v: %v => %v", keyScaleMetricName, metricValueStr, metricValue)
	} else {
		return metricValue, nil
	}
}

func getMetricValue(metricsPrefix, metricName string) (int64, error) {
	key := metricsPrefix + metricName
	if valueStr, ok := getValueFromRedisServer(key); !ok {
		return -1, status.Errorf(codes.InvalidArgument, "invalid %v: %v => %v", keyScaleMetricName, key, valueStr)
	} else if metricValue, err := parseMetricValue(valueStr); err != nil {
		return -1, err
	} else {
		return metricValue, nil
	}
}

func getBytesTotal(metricsPrefix string) (int64, error) {
	if bytesIn, err := getMetricValue(metricsPrefix, keyScaleMetricBytesIn); err != nil {
		return -1, err
	} else if bytesOut, err := getMetricValue(metricsPrefix, keyScaleMetricBytesOut); err != nil {
		return -1, err
	} else {
		bytesTotal := bytesIn + bytesOut
		return bytesTotal, nil
	}
}

func getNumRequestsInOut(metricsPrefix string) (int64, error) {
	if numRequestsIn, err := getMetricValue(metricsPrefix, keyScaleMetricNumRequestsIn); err != nil {
		return -1, err
	} else if numRequestsOut, err := getMetricValue(metricsPrefix, keyScaleMetricNumRequestsOut); err != nil {
		return -1, err
	} else {
		numRequestsInOut := numRequestsIn + numRequestsOut
		return numRequestsInOut, nil
	}
}

func getNumRequestsTotal(metricsPrefix string) (int64, error) {
	if numRequestsInOut, err := getNumRequestsInOut(metricsPrefix); err != nil {
		return -1, err
	} else if numRequestsMisc, err := getMetricValue(metricsPrefix, keyScaleMetricNumRequestsMisc); err != nil {
		return -1, err
	} else {
		numRequestsTotal := numRequestsInOut + numRequestsMisc
		return numRequestsTotal, nil
	}
}

func getMetric(metadata map[string]string) (metric, error) {
	log.Println("getting metric {name, value}")

	var scaleMetricValue int64 = 0
	var err error = nil

	metricsPrefix := getMetricsPrefix(metadata)
	scaleMetricName := getValueFromScalerMetadata(metadata, keyScaleMetricName, defualtScaleMetricName)

	switch strings.ToLower(scaleMetricName) {
	case keyScaleMetricBytesTotal:
		scaleMetricValue, err = getBytesTotal(metricsPrefix)
	case keyScaleMetricNumRequestsInOut:
		scaleMetricValue, err = getNumRequestsInOut(metricsPrefix)
	case keyScaleMetricNumRequestsTotal:
		scaleMetricValue, err = getNumRequestsTotal(metricsPrefix)
	default:
		scaleMetricValue, err = getMetricValue(metricsPrefix, scaleMetricName)
	}

	if err != nil {
		log.Printf("error while getting metric %v [%v]", scaleMetricName, err.Error())
		return metric{}, err
	}

	log.Printf("returning metrics: { metric name: %v, metric value: %v }", scaleMetricName, scaleMetricValue)

	return metric{scaleMetricName, scaleMetricValue}, nil
}
