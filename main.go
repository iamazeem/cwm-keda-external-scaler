package main

import (
	"context"
	_ "fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	pb "github.com/iamAzeem/cwm-keda-external-scaler/externalscaler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	_ "github.com/go-redis/redis/v8"
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
	if value := strings.Trim(os.Getenv(key), " "); value != "" {
		return value
	} else {
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
	if value, exists := metadata[key]; exists {
		return strings.Trim(value, " ")
	} else {
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

	// TODO: custom configuration file via `KUBECONFIG` environment variable

	log.Println("getting in-cluster configuration")

	config, err := rest.InClusterConfig()
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

	log.Printf("found %v deployments in '%v' namespace", len(deploymentList.Items), namespaceName)

	podList, err := clientset.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return -1, status.Error(codes.Internal, err.Error())
	}

	log.Printf("found %v pods in namespace '%v'", len(podList.Items), namespaceName)

	// count the total number of pods in the namespace
	// depending on their deployment name's prefix
	// if the comma-separated list of deployment names has been provided
	// otherwise, return the total number of pods in the current namespace
	pods := 0
	if deploymentNames = strings.TrimSpace(deploymentNames); deploymentNames != "" {
		for _, deploymentName := range strings.Split(deploymentNames, ",") {
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.GetName(), strings.TrimSpace(deploymentName)) {
					log.Printf("'%v' pod found in '%v' deployment", pod.GetName(), deploymentName)
					pods++
				}
			}
		}

		log.Printf("total %v pods found in '%v' deployment(s)", pods, deploymentNames)
	} else {
		log.Printf("invalid/empty list of deployement names! returing pods in '%v' namespace", namespaceName)

		pods = len(podList.Items)
	}

	log.Printf("<< getNoOfPods | pods: %v", pods)

	return pods, nil
}

// External Scaler

type externalScalerServer struct{}

func (s *externalScalerServer) IsActive(ctx context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	log.Println(">> IsActive")

	// cfg := getLocalConfig(in)

	// Is active - will be based on isActiveTtlSeconds and LAST_UPDATE_PREFIX_TEMPLATE

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
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf(">> gRPC server started listening on %v", grpcAddress)

	grpcServer := grpc.NewServer()
	pb.RegisterExternalScalerServer(grpcServer, &externalScalerServer{})
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
