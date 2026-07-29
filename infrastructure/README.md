# Local platform

The lab uses a fixed Docker network so MetalLB can expose stable local
addresses. The commands stay explicit: no Kubernetes operation is hidden in the
Mise tasks.

## Prerequisites

- Docker-compatible runtime
- mise

The root `mise.toml` installs the pinned Kind, kubectl, Helm, and Kustomize
versions used by the lab.

Use the standalone `kustomize` binary. Older versions embedded in `kubectl`
are not compatible with Helm 4.

## Create the cluster

```sh
docker network create --driver bridge --subnet 172.28.0.0/16 lab404
KIND_EXPERIMENTAL_DOCKER_NETWORK=lab404 kind create cluster \
  --config infrastructure/cluster.yaml \
  --image kindest/node:v1.35.0@sha256:452d707d4862f52530247495d180205e029056831160e22870e37e3f6c1ac31f
kubectl label node lab404-worker2 lab404.io/workload=stateful
kubectl taint node lab404-worker2 workload=stateful:NoSchedule
```

If `172.28.0.0/16` conflicts with a host network, select another private `/16`
and update both the Docker network and
`infrastructure/configuration/metallb-address-pool.yaml`.

## Install the platform controllers

```sh
kustomize build --enable-helm infrastructure/controllers |
  kubectl apply --server-side -f -
kubectl wait --namespace metallb-system \
  --for=condition=Available deployment/metallb-controller \
  --timeout=5m
kubectl wait --namespace envoy-gateway-system \
  --for=condition=Available deployment/envoy-gateway \
  --timeout=5m
kubectl wait --namespace observability \
  --for=condition=Available deployment/otel-collector \
  --timeout=5m
```

Kustomize renders the pinned Helm charts as ordinary Kubernetes manifests. It
does not create Helm releases.

## Apply the platform configuration

The custom resources are applied only after their controllers and CRDs are
ready.

```sh
kubectl apply -k infrastructure/configuration
```

## Deploy the application

Update the two image names in
`kubernetes/overlays/local/kustomization.yaml`, then inspect and apply the
rendered resources.

```sh
kubectl kustomize kubernetes/overlays/local
kubectl apply -k kubernetes/overlays/local
kubectl wait --namespace lab404 \
  --for=condition=Available deployment \
  --all \
  --timeout=5m
```

Find the application address:

```sh
kubectl get gateway --namespace lab404 lab404
kubectl get service --all-namespaces
```

Confirm that both services export telemetry:

```sh
kubectl logs --namespace observability deployment/otel-collector
```
