package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/iamAzeem/cwm-keda-external-scaler/externalscaler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

// Cluster configuration

func getNoOfPods(metadata map[string]string) (int, error) {
	log.Println(">> getNoOfPods")

	log.Println("creating kubernetes REST client")

	kubeconfig := getEnv("KUBECONFIG", "")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	namespaceName := getValueFromScalerMetadata(metadata, "namespaceName", defaultNamespaceName)

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

	deploymentNames := getValueFromScalerMetadata(metadata, "deploymentNames", defaultDeploymentNames)

	pods := 0
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

		pods = len(podList.Items)
	}

	log.Printf("<< getNoOfPods | pods: %v", pods)

	return pods, nil
}

// Redis

var (
	rdb *redis.Client = nil
)

func connectToRedisServer() bool {
	log.Printf("establishing connection with Redis server")

	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	address := redisHost + ":" + redisPort

	// check if the existing Redis server's address <host:port> changed
	// close the existing connection and cleanup
	// and try to connect with the new Redis server
	if rdb != nil && address != rdb.Options().Addr {
		log.Printf("address of Redis server changed from %v to %v", rdb.Options().Addr, address)
		log.Printf("previous Redis connection will be closed and the new one will be established")

		rdb.Close()
		rdb = nil
	}

	// create new Redis client if one does not exist already
	if rdb == nil {
		rdb = redis.NewClient(&redis.Options{
			Addr:     address,
			Password: "", // no password set
			DB:       0,  // use default DB
		})
	}

	if !pingRedisServer() {
		rdb.Close()
		rdb = nil
		return false
	}

	log.Printf("successful connection with Redis server [%v]", address)

	return true
}

func pingRedisServer() bool {
	log.Println("pinging Redis server")

	val, err := rdb.Ping(rdb.Context()).Result()
	switch {
	case err == redis.Nil:
		return false
	case err != nil:
		log.Printf("PING call failed! %v", err.Error())
		return false
	case val == "":
		log.Println("empty value for 'PING'")
		return false
	case strings.ToUpper(val) != "PONG":
		log.Println("PING != PONG")
		return false
	}

	log.Printf("Redis server replied: '%v'", val)

	return true
}

func getValueFromRedisServer(key string) (string, bool) {
	log.Printf("getting value for '%v' key from Redis server", key)

	if !connectToRedisServer() {
		log.Println("could not connect with Redis server")
		return "", false
	}

	val, err := rdb.Get(rdb.Context(), key).Result()
	switch {
	case err == redis.Nil:
		log.Printf("'%v' key does not exist", key)
		return val, false
	case err != nil:
		log.Printf("get call failed for '%v'! %v", key, err.Error())
		return val, false
	case val == "":
		log.Printf("empty value for '%v'", key)
		return val, false
	}

	log.Printf("Redis server returned: '%v'", val)

	return val, true
}

// Utility functions

func getMetricSpec(metadata map[string]string) (string, int64, error) {
	log.Println("getting metric spec {metric name, target value}")

	scaleMetricName := getValueFromScalerMetadata(metadata, keyScaleMetricName, defualtScaleMetricName)

	targetValueStr := getValueFromScalerMetadata(metadata, keyTargetValue, defaultTargetValue)
	targetValue, err := strconv.ParseInt(targetValueStr, 10, 64)
	if err != nil {
		return "", 0, status.Errorf(codes.InvalidArgument, "could not get metadata value for %v. %v", keyTargetValue, err.Error())
	} else if targetValue < 0 {
		return "", 0, status.Errorf(codes.InvalidArgument, "invalid value: %v => %v", keyTargetValue, targetValue)
	}

	log.Printf("returning metric spec: { metric name: %v, target value: %v }", scaleMetricName, targetValue)

	return scaleMetricName, targetValue, nil
}

func getMetrics(metadata map[string]string) (string, int64, error) {
	log.Println("getting metrics {metric name, metric value}")

	scaleMetricName := getValueFromScalerMetadata(metadata, keyScaleMetricName, defualtScaleMetricName)
	scaleMetricValueStr, isValidMetricValue := getValueFromRedisServer(scaleMetricName)
	if !isValidMetricValue {
		return "", 0, status.Errorf(codes.Internal, "invalid %v: %v => %v", keyScaleMetricName, scaleMetricName, scaleMetricValueStr)
	}

	scaleMetricValue, err := strconv.ParseInt(scaleMetricValueStr, 10, 64)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "invalid %v: %v => %v [%v]", keyScaleMetricName, scaleMetricName, scaleMetricValue, err.Error())
	} else if scaleMetricValue < 0 {
		return "", 0, status.Errorf(codes.Internal, "invalid %v: %v => %v", keyScaleMetricName, scaleMetricName, scaleMetricValue)
	}

	log.Printf("returning metrics: { metric name: %v, metric value: %v }", scaleMetricName, scaleMetricValue)

	return scaleMetricName, scaleMetricValue, nil
}

// External Scaler

var (
	lastMetricValue int64     = 0
	lastTimestamp   time.Time = time.Now()
)

type externalScalerServer struct{}

func (s *externalScalerServer) IsActive(ctx context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	log.Println("IsAcive | checking active status")

	// timestamp := time.Now()

	_lastUpdatePrefix := getLastUpdatPrefix(in.ScalerMetadata)
	metricsPrefix := getMetricsPrefix(in.ScalerMetadata)

	scaleMetricName := getValueFromScalerMetadata(in.ScalerMetadata, keyScaleMetricName, defualtScaleMetricName)

	key := metricsPrefix + ":" + scaleMetricName
	_val, success := getValueFromRedisServer(key)
	if !success {
		return nil, status.Errorf(codes.Internal, "could not get value from Redis server for '%v'", key)
	}

	// lastMetricValue = val

	// isActiveTtlSeconds := getValueFromScalerMetadata(in.ScalerMetadata, "isActiveTtlSeconds", "600")

	return &pb.IsActiveResponse{
		Result: true,
	}, nil
}

func (s *externalScalerServer) StreamIsActive(in *pb.ScaledObjectRef, stream pb.ExternalScaler_StreamIsActiveServer) error {
	return nil
}

func (s *externalScalerServer) GetMetricSpec(_ context.Context, in *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	metricName, targetValue, err := getMetricSpec(in.ScalerMetadata)
	if err != nil {
		return nil, err
	}

	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{{
			MetricName: metricName,
			TargetSize: targetValue,
		}},
	}, nil
}

func (s *externalScalerServer) GetMetrics(_ context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	metricName, metricValue, err := getMetrics(in.ScaledObjectRef.ScalerMetadata)
	if err != nil {
		return nil, err
	}

	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{{
			MetricName:  metricName,
			MetricValue: metricValue,
		}},
	}, nil
}

func main() {
	log.Println(">> starting external scaler")

	grpcAddress := "0.0.0.0:50051"
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err.Error())
	}

	log.Printf(">> gRPC server started listening on %v", grpcAddress)

	grpcServer := grpc.NewServer()
	pb.RegisterExternalScalerServer(grpcServer, &externalScalerServer{})
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err.Error())
	}
}
