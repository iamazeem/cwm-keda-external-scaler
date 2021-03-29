package main

import (
	"log"
	"os"
	"strings"
)

func getEnv(key, defaultValue string) string {
	key = strings.TrimSpace(key)
	log.Printf("getting value from env variable: key = '%v', default = '%v'", key, defaultValue)
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	} else {
		log.Printf("'%v' does not exist! falling back to default value '%v'", key, defaultValue)
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
	log.Printf("getting value from scaler metadata: key = '%v', default = '%v'", key, defaultValue)
	if value, exists := metadata[key]; exists {
		return strings.TrimSpace(value)
	} else {
		log.Printf("'%v' does not exist! falling back to default value '%v'", key, defaultValue)
		return defaultValue
	}
}
