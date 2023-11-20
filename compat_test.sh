#!/usr/bin/env bash

script_name=$0

from_version="0.4.1"
to_version="trunk"

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

step=0

helm install ytsaurus oci://registry-1.docker.io/ytsaurus/ytop-chart --version ${from_version}

until [ $step -eq 30 ] || kubectl get pod | grep "ytsaurus-ytop-chart-controller-manager" | grep "2/2" | grep "Running"; do
    echo "Waiting for controller pods"
    sleep 10
    let step=step+1
done

kubectl apply -f config/samples/${from_version}/cluster_v1_minikube.yaml

let step=0
until [ $step -eq 60 ] || kubectl get ytsaurus | grep "Running"; do
    echo "Waiting for ytsaurus is Running"
    sleep 10
    let step=step+1
done

if [[ "$to_version" == "trunk" ]]; then
    docker build -t ytsaurus/k8s-operator:0.0.0-alpha .
    kind load docker-image ytsaurus/k8s-operator:0.0.0-alpha
    helm upgrade ytsaurus ytop-chart
else
    helm upgrade ytsaurus --install oci://docker.io/ytsaurus/ytop-chart --version ${to_version}
fi

sleep 10

until [ $step -eq 30 ] || kubectl get pod | grep "ytsaurus-ytop-chart-controller-manager" | grep "2/2" | grep "Running"; do
    echo "Waiting for controller pods"
    sleep 10
    let step=step+1
done

sleep 20

let step=0
until [ $step -eq 60 ] || kubectl get ytsaurus | grep "Running"; do
    echo "Waiting for ytsaurus is Running again"
    sleep 10
    let step=step+1
done

helm uninstall ytsaurus
