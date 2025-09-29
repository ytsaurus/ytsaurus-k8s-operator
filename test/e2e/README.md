# YTsaurus operator e2e test

## General
Source of truth of configuration and commands to run e2e tests is in the [e2e Github action](https://github.com/ytsaurus/ytsaurus-k8s-operator/blob/main/.github/workflows/subflow_run_e2e_tests.yaml#L34-L49).
The flow:
1) create k8s ([Kind](https://kind.sigs.k8s.io/)) cluster
2) build the YTsaurus operator helm chart
3) install helm chart in the Kind cluster
4) preload test YTsaurus images in kind so pods wouldn't have to download them in tests every time
5) run e2e tests with `make test-e2e` or `make test-e2e-env` (using environment variable)

## Enabling E2E Tests

E2E tests can be enabled in two ways:

1. **CLI flag**: Use `make test-e2e` which passes the `--enable-e2e` flag
2. **Environment variable**: Set `YTOP_ENABLE_E2E=true` and run tests with `make test-e2e-env` or directly with ginkgo

The environment variable approach is useful when you want to enable e2e tests without modifying command line arguments, for example in CI/CD pipelines or when running tests through IDEs.

## Development
In the development, kind cluster (with preloaded images) can be created once and reused between e2e runs. 

Also, it is possible to run the YTsaurus operator main process outside of k8s cluster, for example, to be able to use a debugger. To achieve that use `make install` to install only CRD's instead of `make helm-kind-install` which would install helm chart. Also to have such a configuration, a developer should set `E2E_HTTP_PROXY_INTERNAL_PORT` env variable (for example `E2E_HTTP_PROXY_INTERNAL_PORT=9000`) 
and `YTOP_PROXY` env variable with matching port (for example `YTOP_PROXY=127.0.0.1:9000`) before running the e2e tests.   
This way HTTP proxies of YTsaurus clusters will listen the specified port, and the YTsaurus operator code launched outside of the k8s cluster will use this port to connect to the proxy.

### Note for the MacOS users
One needs to publish ports in Kind's docker container to make the specified port available outside of Kind on MacOS.
To achieve that, the next approach can be used:
1. create Kind cluster with some ports forwarded, for example
```
# kind-config.yaml

apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30000
        hostPort: 9000
      - containerPort: 30001
        hostPort: 9001
      - containerPort: 30010
        hostPort: 9010
      - containerPort: 30011
        hostPort: 9011
      - containerPort: 30020
        hostPort: 9020
      - containerPort: 30021
        hostPort: 9021
```
```
# create_kind.sh
kind create cluster --config=kind-config.yaml
# also cert manager required for operator to work
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
```
2. Configure env variables
```
YTOP_PROXY=127.0.0.1:9000
E2E_HTTP_PROXY_INTERNAL_PORT=30000
```

RPC proxy can be configured in a similar way, if needed `E2E_YT_RPC_PROXY=127.0.0.1:9001; E2E_RPC_PROXY_INTERNAL_PORT=30001`.
