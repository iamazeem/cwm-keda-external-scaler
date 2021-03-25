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
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Global configuration

type globalConfig struct {
	redisHost                string
	redisPort                string
	lastUpdatePrefixTemplate string
	metricsPrefixTemplate    string
	kubeConfig               string
}

func getEnv(key, defaultValue string) string {
	key = strings.TrimSpace(key)
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	} else {
		log.Printf("'%v' does not exist! falling back to default value '%v'", key, defaultValue)
		return defaultValue
	}
}

func getGlobalConfig() *globalConfig {
	return &globalConfig{
		redisHost:                getEnv("REDIS_HOST", "0.0.0.0"),
		redisPort:                getEnv("REDIS_PORT", "6379"),
		lastUpdatePrefixTemplate: getEnv("LAST_UPDATE_PREFIX_TEMPLATE", "deploymentid:last_action"),
		metricsPrefixTemplate:    getEnv("METRICS_PREFIX_TEMPLATE", "deploymentid:last_action"),
		kubeConfig:               getEnv("KUBECONFIG", "~/.kube/config"),
	}
}

// Local configuration

type localConfig struct {
	deploymentid       string
	isActiveTtlSeconds int
	scaleMetricName    string
	scalePeriodSeconds int
	namespaceName      string
	deploymentNames    []string
	targetValue        int
}

func getScalerMetadata(metadata map[string]string, key, defaultValue string) string {
	key = strings.TrimSpace(key)
	if value, exists := metadata[key]; exists {
		return strings.TrimSpace(value)
	} else {
		log.Printf("'%v' does not exist! falling back to default value '%v'", key, defaultValue)
		return defaultValue
	}
}

func getLocalConfig(scaledObject *pb.ScaledObjectRef) *localConfig {
	metadata := scaledObject.ScalerMetadata

	cfg := &localConfig{}
	cfg.deploymentid = getScalerMetadata(metadata, "deploymentid", "deploymentid")

	isActiveTtlSeconds, _ := strconv.Atoi(getScalerMetadata(metadata, "isActiveTtlSeconds", "300"))
	cfg.isActiveTtlSeconds = isActiveTtlSeconds

	cfg.scaleMetricName = getScalerMetadata(metadata, "scaleMetricName", "bytes_in")

	scalePeriodSeconds, _ := strconv.Atoi(getScalerMetadata(metadata, "scalePeriodSeconds", "300"))
	cfg.scalePeriodSeconds = scalePeriodSeconds

	cfg.namespaceName = getScalerMetadata(metadata, "namespaceName", "default")

	// handle comman-separated list of deployment names
	deploymentNames := getScalerMetadata(metadata, "deploymentNames", "")
	if deploymentNames != "" {
		names := strings.Split(deploymentNames, ",")
		for i, n := range names {
			names[i] = strings.Trim(n, " ")
		}
		cfg.deploymentNames = names
	}

	targetValue, _ := strconv.Atoi(getScalerMetadata(metadata, "targetValue", "10"))
	cfg.targetValue = targetValue

	log.Println(cfg.deploymentid)

	return cfg
}

func getNoOfPods(namespaceName, deploymentNames string) (int, error) {
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

func connectToRedisServer() *redis.Client {
	log.Printf("establishing connection with Redis server")

	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	address := redisHost + ":" + redisPort

	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	if !pingRedisServer(rdb) {
		rdb.Close()
		return nil
	}

	return rdb
}

func pingRedisServer(rdb *redis.Client) bool {
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

	rdb := connectToRedisServer()
	if rdb == nil {
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

// External Scaler

var (
	lastMetricValue string    = ""
	lastTimestamp   time.Time = time.Now()
)

type externalScalerServer struct{}

func (s *externalScalerServer) IsActive(ctx context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	log.Println(">> IsActive")

	// timestamp := time.Now()

	lastUpdatePrefix := getEnv("LAST_UPDATE_PREFIX_TEMPLATE", "deploymentid:last_action")
	deploymentid := getScalerMetadata(in.ScalerMetadata, "deploymentid", "deploymentid")
	scaleMetricName := getScalerMetadata(in.ScalerMetadata, "scaleMetricName", "bytes_out")

	key := lastUpdatePrefix + ":" + deploymentid + ":" + scaleMetricName
	val, success := getValueFromRedisServer(key)
	if !success {
		return nil, status.Errorf(codes.Internal, "could not get value from Redis server for '%v'", key)
	}

	lastMetricValue = val

	// isActiveTtlSeconds := getScalerMetadata(in.ScalerMetadata, "isActiveTtlSeconds", "600")

	return &pb.IsActiveResponse{
		Result: true,
	}, nil
}

func (s *externalScalerServer) StreamIsActive(in *pb.ScaledObjectRef, stream pb.ExternalScaler_StreamIsActiveServer) error {
	return nil
}

func (s *externalScalerServer) GetMetricSpec(ctx context.Context, in *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	log.Println(">> GetMetricSpec")

	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{{
			MetricName: "", // scaleMetricName
			TargetSize: 10, // targetValue
		}},
	}, nil
}

func (s *externalScalerServer) GetMetrics(ctx context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	log.Println(">> GetMetrics")

	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{{
			MetricName:  "", // scaleMetricName
			MetricValue: 10, // scaleMetricValue
		}},
	}, nil
}

func (s *externalScalerServer) Close(ctx context.Context, scaledObjectRef *pb.ScaledObjectRef) (*empty.Empty, error) {
	log.Println(">> Close")

	out := &empty.Empty{}
	return out, nil
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
