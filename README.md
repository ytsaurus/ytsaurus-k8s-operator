# ytsaurus-k8s-operator

YTsaurus is a distributed storage and processing platform for big data with support for MapReduce model, a distributed file system and a NoSQL key-value database.

This operator helps you to deploy YTsaurus using Kubernetes.

## Description
Currently available in alpha-version and is capable to deploy a new YTsaurus cluster from scratch, primarily for testing purposes. Also can perform automated cluster upgrades with downtime.


## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

You can install pre-built versions of operator via [helm chart](https://hub.docker.com/r/ytsaurus/ytop-chart).

Next you need to [prepare the Ytsaurus specification](https://ytsaurus.tech/docs/en/admin-guide/prepare-spec), see provided [samples](config/samples) and [API Reference](docs/api.md).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/cluster_v1_demo.yaml
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/ytsaurus-k8s-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/ytsaurus-k8s-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
We are glad to welcome new contributors!

1. Please read the [contributor's guide](CONTRIBUTING.md).
2. We can accept your work to YTsaurus after you have signed contributor's license agreement (aka CLA).
3. Please don't forget to add a note to your pull request, that you agree to the terms of the CLA.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

