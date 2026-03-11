# ytop-chart

Helm chart for the [YTsaurus Kubernetes operator](https://github.com/ytsaurus/ytsaurus-k8s-operator) that deploys and manages [YTsaurus](https://ytsaurus.tech) clusters.

## Prerequisites

- Kubernetes 1.21+
- Helm 3.8+
- [cert-manager](https://cert-manager.io/) installed in the cluster (required for webhook TLS certificates)

## Installation

### From OCI registry (recommended)

```sh
helm install ytsaurus-operator oci://registry-1.docker.io/ytsaurus/ytop-chart
```

To install a specific version:

```sh
helm install ytsaurus-operator oci://registry-1.docker.io/ytsaurus/ytop-chart --version <version>
```

### From source

```sh
make helm-install
```

## Uninstallation

```sh
helm uninstall ytsaurus-operator
```

> **Note:** By default (`crds.keep: true`) CRDs are **not** removed when the chart is uninstalled.
> This protects YTsaurus cluster data from accidental deletion.
> To remove CRDs explicitly, first list the CRDs created by this release, for example:
> `kubectl get crds -l app.kubernetes.io/managed-by=Helm,app.kubernetes.io/instance=ytsaurus-operator`
> and then delete the specific CRDs you want to remove by name, for example:
> `kubectl delete crd <crd-name-1> <crd-name-2>`.

## Configuration

The following table lists the configurable parameters of the chart and their default values.

### Custom CA root bundle

| Parameter | Description | Default |
|-----------|-------------|---------|
| `caRootBundle` | Custom CA root certificate bundle mounted into the manager container. Supported kinds: `ConfigMap`, `Secret`. | `{}` (disabled) |
| `caRootBundle.kind` | Resource kind: `ConfigMap` or `Secret`. | — |
| `caRootBundle.name` | Name of the resource containing the CA bundle. | — |
| `caRootBundle.key` | Key inside the resource that holds the PEM-encoded CA certificates. | — |

### CRDs

| Parameter | Description | Default |
|-----------|-------------|---------|
| `crds.enabled` | Install CRDs together with the chart. Set to `false` when CRDs are managed separately. | `true` |
| `crds.keep` | Keep CRDs when the chart is uninstalled. Prevents accidental data loss. | `true` |

### Controller manager

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controllerManager.replicas` | Number of controller manager replicas. Only one replica is active at a time due to leader election. | `1` |
| `controllerManager.revisionHistoryLimit` | Number of old ReplicaSets to retain for rollback. | `10` |
| `controllerManager.nodeSelector` | Node labels required for scheduling the manager pod. | `{}` |
| `controllerManager.affinity` | Affinity rules for the manager pod. | `{}` |
| `controllerManager.tolerations` | Tolerations for the manager pod to schedule on tainted nodes. | `[]` |
| `controllerManager.strategy` | Deployment update strategy. | `{type: RollingUpdate}` |
| `controllerManager.serviceAccount.annotations` | Extra annotations on the ServiceAccount (e.g. for Workload Identity / IRSA). | `{}` |

#### Manager container

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controllerManager.manager.image.repository` | Container image repository. | `ytsaurus/k8s-operator` |
| `controllerManager.manager.image.tag` | Container image tag. Defaults to the chart `appVersion` when empty. | `""` |
| `controllerManager.manager.args` | Extra arguments passed to the operator binary. | See `values.yaml` |
| `controllerManager.manager.env` | Environment variables for the manager container. | See `values.yaml` |
| `controllerManager.manager.env.GOMAXPROCS` | Limits the number of OS threads used by the Go runtime. | `1` |
| `controllerManager.manager.env.YT_LOG_LEVEL` | Log level for the YTsaurus client library (`DEBUG`, `INFO`, `WARN`, `ERROR`). | `DEBUG` |
| `controllerManager.manager.resources` | CPU and memory resource requests/limits. | See `values.yaml` |
| `controllerManager.manager.containerSecurityContext` | Security context for the manager container. | Drop `ALL` capabilities |
| `controllerManager.manager.instanceScope` | When `true`, the operator watches only resources whose `ytsaurus.tech/operator-instance` label matches the Helm release name. Enables running multiple operator instances in a single cluster. | `false` |
| `controllerManager.manager.namespacedScope` | When `true`, the operator watches resources only in its own namespace. | `false` |

### Metrics service

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metricsService.type` | Kubernetes Service type for the metrics endpoint. | `ClusterIP` |
| `metricsService.ports` | Port definitions for the metrics service. | Port `8080` |

### Webhook service

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhookService.type` | Kubernetes Service type for the validating webhook server. | `ClusterIP` |
| `webhookService.ports` | Port definitions for the webhook service. | Port `443 → 9443` |

### pprof

| Parameter | Description | Default |
|-----------|-------------|---------|
| `pprof.enabled` | Enable the Go pprof profiling HTTP endpoint. Enable only for debugging. | `false` |
| `pprof.address` | Address the pprof server listens on. | `localhost` |
| `pprof.port` | Port the pprof server listens on. | `6060` |

### Other

| Parameter | Description | Default |
|-----------|-------------|---------|
| `kubernetesClusterDomain` | DNS domain of the Kubernetes cluster, used to build fully qualified service names. | `cluster.local` |

## Multi-instance deployment

The operator supports running multiple instances in the same cluster, with each instance managing a distinct set of YTsaurus resources.

To enable this mode, set `controllerManager.manager.instanceScope: true`.
Each resource that should be managed by a specific operator instance must carry the label:

```yaml
ytsaurus.tech/operator-instance: <helm-release-name>
```

## Namespace-scoped deployment

To restrict the operator to watch resources only in its own namespace, set `controllerManager.manager.namespacedScope: true`.
This is useful when cluster-wide RBAC permissions are not available.

## Custom CA certificates

If your Kubernetes cluster or YTsaurus clusters use a custom or self-signed CA, you can inject the CA bundle into the operator by configuring `caRootBundle`:

```yaml
caRootBundle:
  kind: ConfigMap
  name: ca-root-bundle
  key: ca-certificates.crt
```

The bundle is mounted at `/etc/ssl/certs/ca-certificates.crt` inside the manager container.

## Upgrading

After upgrading the chart, the operator will automatically reconcile all managed YTsaurus clusters.

To upgrade to a new chart version:

```sh
helm upgrade ytsaurus-operator oci://registry-1.docker.io/ytsaurus/ytop-chart --version <new-version>
```

## Further reading

- [YTsaurus operator documentation](https://ytsaurus.tech/docs/en/admin-guide/prepare-spec)
- [API Reference](https://github.com/ytsaurus/ytsaurus-k8s-operator/blob/main/docs/api.md)
- [Sample cluster configurations](https://github.com/ytsaurus/ytsaurus-k8s-operator/tree/main/config/samples)
