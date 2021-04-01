# cwm-keda-external-scaler

![GitHub release (latest by date)](https://img.shields.io/github/v/release/iamAzeem/cwm-keda-external-scaler)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/iamAzeem/cwm-keda-external-scaler/blob/main/LICENSE)

![Lines of code](https://img.shields.io/tokei/lines/github/iamAzeem/cwm-keda-external-scaler?label=LOC)
![GitHub code size in bytes](https://img.shields.io/github/languages/code-size/iamAzeem/cwm-keda-external-scaler)
![GitHub repo size](https://img.shields.io/github/repo-size/iamAzeem/cwm-keda-external-scaler)

## Overview

CWM KEDA external scaler for scaling workers.

```text
                                      CONFIGURATION (global and local)
                                    ------------------------------------
                                    Env Variables: { REDIS_HOST, ... }
            {metrics}               ScaledObject : { deploymentid, ... }
                |                                  |
                |                                  |
                |                                  |
                |                                  |
      +---------v---------+              +---------v---------+
      |                   |   {metric}   |                   |
      |    Redis Server   --------------->  External Scaler  |
      |                   |              |                   |
      +-------------------+              +---------|---------+
                                                   |
                                                   |
                                                   |
                                         +---------v---------+
                                         |                   |
                                         |     Kubernetes    |
                                         |                   |
                                         +---------|---------+
                                                   |
                                                   |  scale
                                                   |
                                         +---------v---------+
                                         |                   |
                                         |  Target Resource  |
                                         |                   |
                                         +-------------------+
```

## Configuration

### Global Configuration: Environment Variables

| Environment Variable            | Description                           |
|:--------------------------------|:--------------------------------------|
| `REDIS_HOST`                    | ip/host of the Redis metrics server   |
| `REDIS_PORT`                    | port of the Redis metrics server      |
| `LAST_UPDATE_PREFIX_TEMPLATE`   | timestamp of last update              |
| `METRICS_PREFIX_TEMPLATE`       | prefix to get the metrics from        |

### Local Configuration: Metadata in ScaledObject

Here is the generic YAML format of a ScaledObject:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: <scaledobject-name>
  namespace: <scaledobject-namespace>
spec:
  scaleTargetRef:
    name: <deployment-name>
  triggers:
    - type: external
      metadata:                       # <<< local configuration >>> #
        scalerAddress: <host:port>
        key1: <value1>
        key2: <value2>
```

The local configuration will be under `metadata`:

```yaml
# ...
spec:
  # ...
  triggers:
    - type: external
      metadata:
        # <<< local confiugration >>>
```

The following table lists the supported local configuration:

| Configuration (Key)           | Description                           |
|:------------------------------|:--------------------------------------|
| `deploymentid`                | value to replace in the prefix templates |
| `isActiveTtlSeconds`          | seconds since last update to consider the workload as active |
| `scaleMetricName`             | metric for scaling (listed below)     |
| `scalePeriodSeconds`          | retention time for the metric value   |
| `targetValue`                 | target value reported by the autoscaler |

**NOTE**: The `deploymentNames` may be a comma-separated list of names.

Supported options for `scaleMetricName`:

- `bytes_in`
- `bytes_out`
- `num_requests_in`
- `num_requests_out`
- `num_requests_misc`
- `bytes_total` (`bytes_in` + `bytes_out`)
- `num_requests_in_out` (`num_requests_in` + `num_requests_out`)
- `num_requests_total` (`num_requests_in` + `num_requests_out` + `num_requests_misc`)

### Sample Configuration

Here's the
[configuration](https://keda.sh/docs/2.1/concepts/scaling-deployments/#scaledobject-spec)
format of a `ScaledObject`:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {scaled-object-name}
spec:
  scaleTargetRef:
    apiVersion:    {api-version-of-target-resource}  # Optional. Default: apps/v1
    kind:          {kind-of-target-resource}         # Optional. Default: Deployment
    name:          {name-of-target-resource}         # Mandatory. Must be in the same namespace as the ScaledObject
    envSourceContainerName: {container-name}         # Optional. Default: .spec.template.spec.containers[0]
  pollingInterval: 30                                # Optional. Default: 30 seconds
  cooldownPeriod:  300                               # Optional. Default: 300 seconds
  minReplicaCount: 0                                 # Optional. Default: 0
  maxReplicaCount: 100                               # Optional. Default: 100
  advanced:                                          # Optional. Section to specify advanced options
    restoreToOriginalReplicaCount: true/false        # Optional. Default: false
    horizontalPodAutoscalerConfig:                   # Optional. Section to specify HPA related options
      behavior:                                      # Optional. Use to modify HPA's scaling behavior
        scaleDown:
          stabilizationWindowSeconds: 300
          policies:
          - type: Percent
            value: 100
            periodSeconds: 15
  triggers:
  # {list of triggers to activate scaling of the target resource}
```

Assuming that the global configuration via environment variables has properly
been set, our external scaler (`cwm-keda-external-scaler`) can be configured
under `triggers` like this:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name:                     {scaled-object-name}
spec:
  scaleTargetRef:
    name:                   {name-of-target-resource}
  pollingInterval: 10
  triggers:
    - type: external
      metadata:
        scalerAddress:      {host:port}
        deploymentid:       {deployment-id}
        isActiveTtlSeconds: {seconds}
        scaleMetricName:    {supported-metric-name}
        scalePeriodSeconds: {seconds}
        targetValue:        {target-value}
```

## Build Docker Image

```shell
docker build -t cwm-keda-external-scaler:latest .
```

## Testing

### Deploy

Terminal-1: Watch namespaces

```shell
watch -x kubectl get all --all-namespaces
```

Terminal-2: Apply test deployment

```shell
kubectl apply -f ./deploy.yaml
```

Terminal-3: Check logs of `pod/keda-operator-*` in `keda` namespace

```shell
kubectl logs -f -n keda pod/keda-operator-*
```

Terminal-4: Check logs of `pod/keda-operator-metrics-apiserver-*` in `keda` namespace

```shell
kubectl logs -f pod/keda-operator-metrics-apiserver-* -n keda
```

Terminal-5: Check logs of the custom external scaler

```shell
kubectl logs -f pod/cwm-keda-external-scaler-* -n <namespace>
```

**NOTE**: The trailing `*` in above `pod/<pod-name>-*` format denotes the actual
complete name of the pod.

## Contribute

- Fork the project.
- Check out the latest `main` branch.
- Create a feature or bugfix branch from `main`.
- Commit and push your changes.
- Make sure to add tests.
- Test locally.
- Submit the PR.

## License

[MIT](LICENSE)
