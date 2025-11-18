#!/bin/bash

script_name=$0

from_version="0.27.0"
to_version="trunk"
helm_chart="oci://ghcr.io/ytsaurus/ytop-chart"
cluster_spec="config/samples/cluster_v1_local.yaml"
cluster_name="minisaurus"

KIND="go tool -modfile=tool/go.mod kind"

print_usage() {
    cat << EOF
Usage: $script_name [-h|--help]
                    [--from-version <operator-version> (default: $from_version)]
                    [--to-version <operator-version> (default: $to_version)]
EOF
    exit 1
}

# Parse options
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        --from-version)
        from_version="$2"
        shift 2
        ;;
        --to-version)
        to_version="$2"
        shift 2
        ;;
        *)  # unknown option
        echo "Unknown argument $1"
        print_usage
        ;;
    esac
done

set -eux -o pipefail

helm install ytsaurus ${helm_chart} --version ${from_version} --wait

kubectl get crd -l app.kubernetes.io/part-of=ytsaurus-k8s-operator -L app.kubernetes.io/version

kubectl create -f ${cluster_spec}

kubectl wait ytsaurus/${cluster_name} --for=jsonpath='{.status.state}=Running' --timeout=10m

kubectl get ytsaurus/${cluster_name} -o jsonpath-as-json="{.status.conditions[?(@.type=='OperatorVersion')].message}"

if [[ "$to_version" == "trunk" ]]; then
    docker build -t ytsaurus/k8s-operator:trunk --build-arg VERSION="trunk" .
    $KIND load docker-image ytsaurus/k8s-operator:trunk
    helm upgrade ytsaurus ytop-chart --wait \
        --set controllerManager.manager.image.repository=ytsaurus/k8s-operator \
        --set controllerManager.manager.image.tag=trunk
    kubectl wait ytsaurus/${cluster_name} --for=jsonpath="{.status.conditions[?(@.type=='OperatorVersion')].message}='${to_version}'" --timeout=1m
else
    helm upgrade ytsaurus --install ${helm_chart} --version ${to_version} --wait
fi

kubectl get crd -l app.kubernetes.io/part-of=ytsaurus-k8s-operator -L app.kubernetes.io/version

kubectl wait ytsaurus/${cluster_name} --for=jsonpath='{.status.state}=Running' --timeout=10m

helm uninstall ytsaurus --wait

kubectl get crd -l app.kubernetes.io/part-of=ytsaurus-k8s-operator -L app.kubernetes.io/version

# Current chart version by default does not remove CRDs and clusters at uninstall.
kubectl get crd/ytsaurus.cluster.ytsaurus.tech
kubectl get ytsaurus/${cluster_name}

kubectl delete -f ${cluster_spec} --cascade=foreground

kubectl delete crd -l app.kubernetes.io/part-of=ytsaurus-k8s-operator
