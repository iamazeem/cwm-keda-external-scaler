package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	pb "github.com/iamAzeem/cwm-keda-external-scaler/externalscaler"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type metric struct {
	name  string
	value int64
}

// Utility functions

func getIsActiveTtlSeconds(metadata map[string]string) (int64, error) {
	isActiveTtlSecondsStr := getValueFromScalerMetadata(metadata, keyIsActiveTtlSeconds, defaultIsActiveTtlSeconds)
	isActiveTtlSeconds, err := strconv.ParseInt(isActiveTtlSecondsStr, 10, 64)
	if err != nil {
		return -1, status.Errorf(codes.InvalidArgument, "could not get metadata value for %v. %v", keyIsActiveTtlSeconds, err.Error())
	} else if isActiveTtlSeconds < 0 {
		return -1, status.Errorf(codes.InvalidArgument, "invalid value: %v => %v", keyIsActiveTtlSeconds, isActiveTtlSeconds)
	}

	return isActiveTtlSeconds, nil
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
	scalePeriodSeconds, err := strconv.ParseInt(scalePeriodSecondsStr, 10, 64)
	if err != nil {
		return -1, status.Errorf(codes.InvalidArgument, "could not get metadata value for %v. %v", keyIsActiveTtlSeconds, err.Error())
	} else if scalePeriodSeconds < 0 {
		return -1, status.Errorf(codes.InvalidArgument, "invalid value: %v => %v", keyScalePeriodSeconds, scalePeriodSeconds)
	}

	return scalePeriodSeconds, nil
}

func isActive(metadata map[string]string) (bool, error) {
	log.Println("checking active status")

	isActiveTtlSeconds, err := getIsActiveTtlSeconds(metadata)
	if err != nil {
		return false, err
	}

	lastUpdateTime, err := getLastUpdateTime(metadata)
	if err != nil {
		return false, err
	}

	// add metric value in cache
	m, err := getMetric(metadata)
	if err != nil {
		return false, err
	}

	cache.append(m.value)

	// purge metric values from cache older than scale period seconds ago
	scalePeriodSeconds, err := getScalePeriodSeconds(metadata)
	if err != nil {
		return false, err
	}

	cache.purge(scalePeriodSeconds)

	// determine activeness
	active := int64(time.Since(lastUpdateTime).Seconds()) < isActiveTtlSeconds
	log.Printf("isActive: %v", active)

	return active, nil
}

func getMetricSpec(metadata map[string]string) (metric, error) {
	log.Println("getting metric spec {metric name, target value}")

	scaleMetricName := getValueFromScalerMetadata(metadata, keyScaleMetricName, defualtScaleMetricName)

	targetValueStr := getValueFromScalerMetadata(metadata, keyTargetValue, defaultTargetValue)
	targetValue, err := strconv.ParseInt(targetValueStr, 10, 64)
	if err != nil {
		return metric{}, status.Errorf(codes.InvalidArgument, "could not get metadata value for %v. %v", keyTargetValue, err.Error())
	} else if targetValue < 0 {
		return metric{}, status.Errorf(codes.InvalidArgument, "invalid value: %v => %v", keyTargetValue, targetValue)
	}

	log.Printf("returning metric spec: { metric name: %v, target value: %v }", scaleMetricName, targetValue)

	return metric{scaleMetricName, targetValue}, nil
}

func getMetric(metadata map[string]string) (metric, error) {
	log.Println("getting metric {name, value}")

	scaleMetricName := getValueFromScalerMetadata(metadata, keyScaleMetricName, defualtScaleMetricName)
	scaleMetricValueStr, isValidMetricValue := getValueFromRedisServer(scaleMetricName)
	if !isValidMetricValue {
		return metric{}, status.Errorf(codes.Internal, "invalid %v: %v => %v", keyScaleMetricName, scaleMetricName, scaleMetricValueStr)
	}

	scaleMetricValue, err := strconv.ParseInt(scaleMetricValueStr, 10, 64)
	if err != nil {
		return metric{}, status.Errorf(codes.Internal, "invalid %v: %v => %v [%v]", keyScaleMetricName, scaleMetricName, scaleMetricValue, err.Error())
	} else if scaleMetricValue < 0 {
		return metric{}, status.Errorf(codes.Internal, "invalid %v: %v => %v", keyScaleMetricName, scaleMetricName, scaleMetricValue)
	}

	log.Printf("returning metrics: { metric name: %v, metric value: %v }", scaleMetricName, scaleMetricValue)

	return metric{scaleMetricName, scaleMetricValue}, nil
}

func getMetrics(metadata map[string]string) (metric, error) {
	metricData, err := cache.getOldestMetricData()
	if err != nil {
		return metric{}, err
	}

	oldMetricValue := metricData.metricValue

	m, err := getMetric(metadata)
	if err != nil {
		return metric{}, err
	}

	noOfPods, err := getNoOfPods(metadata)
	if err != nil {
		return metric{}, err
	}

	metricValue := (m.value - oldMetricValue) / noOfPods

	return metric{m.name, metricValue}, nil
}

// External Scaler

type externalScalerServer struct{}

func (s *externalScalerServer) IsActive(_ context.Context, in *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	result, err := isActive(in.ScalerMetadata)
	if err != nil {
		return nil, err
	}

	return &pb.IsActiveResponse{
		Result: result,
	}, nil
}

func (s *externalScalerServer) StreamIsActive(in *pb.ScaledObjectRef, stream pb.ExternalScaler_StreamIsActiveServer) error {
	return nil
}

func (s *externalScalerServer) GetMetricSpec(_ context.Context, in *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	m, err := getMetricSpec(in.ScalerMetadata)
	if err != nil {
		return nil, err
	}

	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{{
			MetricName: m.name,
			TargetSize: m.value,
		}},
	}, nil
}

func (s *externalScalerServer) GetMetrics(_ context.Context, in *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	m, err := getMetrics(in.ScaledObjectRef.ScalerMetadata)
	if err != nil {
		return nil, err
	}

	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{{
			MetricName:  m.name,
			MetricValue: m.value,
		}},
	}, nil
}

// main function

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	log.Println("starting external scaler")

	grpcAddress := "0.0.0.0:50051"
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err.Error())
	}

	log.Printf("gRPC server started listening on %v", grpcAddress)

	grpcServer := grpc.NewServer()
	pb.RegisterExternalScalerServer(grpcServer, &externalScalerServer{})
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err.Error())
	}
}
