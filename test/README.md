# Testing

## Prerequisites

| Software      | Version       |
|:-------------:|:-------------:|
| minikube      | v1.12.3       |
| Kubernetes    | v1.16.4       |
| KEDA          | v2.1.0        |

## Important

- The [CI workflow](./../.github/workflows/ci.yml) is kept to minimal. The
  accompanying scripts contain the complete steps. The CI provides the
  environment variables to the scripts wherever required. The scripts also
  contain comments to signify this e.g.
  [push_docker_image.sh](./../bin/push_docker_image.sh).

- The commands/scripts that use Docker/K8s first execute:

  ```shell
  eval "$(minikube -p minikube docker-env)"
  ```

- The minikube's `kubectl` is used throughout via `KUBECTL` bash variable.

  ```bash
  KUBECTL="minikube kubectl --"
  ```

  Using this convention, the relevant version of `kubectl` is automatically
  downloaded and used to match the required Kubernetes version.

## Tests

Please see [run_scaling_tests.sh](./run_scaling_tests.sh) script.

## Publish Docker Image to GitHub Packages

The [push_docker_image.sh](./../bin/push_docker_image.sh) script uses minikube's
docker registry to push the existing built image to GitHub Packages.
