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

	_ "github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

const (
	grpcAddress string = "0.0.0.0:50051"
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

// External Scaler

type externalScalerServer struct{}

func (s *externalScalerServer) IsActive(ctx context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	log.Println(">> IsActive")

	out := &pb.IsActiveResponse{}

	return out, nil
}

func (s *externalScalerServer) StreamIsActive(in *pb.ScaledObjectRef, stream pb.ExternalScaler_StreamIsActiveServer) error {
	log.Println(">> StreamIsActive")

	return nil
}

func (s *externalScalerServer) GetMetricSpec(ctx context.Context, in *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	log.Println(">> GetMetricSpec")

	out := &pb.GetMetricSpecResponse{}

	return out, nil
}

func (s *externalScalerServer) GetMetrics(ctx context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	log.Println(">> GetMetrics")

	out := &pb.GetMetricsResponse{}

	return out, nil
}

func (s *externalScalerServer) Close(ctx context.Context, scaledObjectRef *pb.ScaledObjectRef) (*empty.Empty, error) {
	log.Println(">> Close")

	out := &empty.Empty{}
	return out, nil
}

func main() {
	log.Println(">> starting external scaler")

	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println(">> gRPC server started listening on %v", grpcAddress)

	grpcServer := grpc.NewServer()
	pb.RegisterExternalScalerServer(grpcServer, &externalScalerServer{})
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
