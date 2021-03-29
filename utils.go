package main

import (
	"context"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

func getNoOfPods(metadata map[string]string) (int64, error) {
	log.Println("getting the number of pods")

	log.Println("creating kubernetes REST client")

	kubeconfig := getEnv(keyKubeConfig, "")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	namespaceName := getValueFromScalerMetadata(metadata, keyNamespaceName, defaultNamespaceName)

	log.Printf("getting deployments in '%v' namespace", namespaceName)

	deploymentList, err := clientset.AppsV1().Deployments(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	log.Printf("found %v deployment(s) in '%v' namespace", len(deploymentList.Items), namespaceName)

	podList, err := clientset.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	log.Printf("found %v pod(s) in '%v' namespace", len(podList.Items), namespaceName)

	// count the total number of pods in the namespace
	// depending on their deployment name's prefix
	// if the comma-separated list of deployment names has been provided
	// otherwise, return the total number of pods in the current namespace

	deploymentNames := getValueFromScalerMetadata(metadata, keyDeploymentNames, defaultDeploymentNames)

	var pods int64 = 0
	if deploymentNames = strings.TrimSpace(deploymentNames); deploymentNames != "" {
		for _, deploymentName := range strings.Split(deploymentNames, ",") {
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.GetName(), strings.TrimSpace(deploymentName)) {
					log.Printf("found '%v' pod in '%v' deployment", pod.GetName(), deploymentName)
					pods++
				}
			}
		}

		log.Printf("total %v pod(s) found in '%v' deployment(s)", pods, deploymentNames)
	} else {
		log.Printf("invalid/empty list of deployement names! returing pods in '%v' namespace", namespaceName)

		pods = int64(len(podList.Items))
	}

	log.Printf("number of pods: %v", pods)

	return pods, nil
}
