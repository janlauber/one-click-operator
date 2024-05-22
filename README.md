# One-Click Operator

[![CodeFactor](https://www.codefactor.io/repository/github/janlauber/one-click-operator/badge)](https://www.codefactor.io/repository/github/janlauber/one-click-operator)

## Description

The purpose of this Kubernetes operator is to significantly streamline the deployment process of single-container applications on Kubernetes. Traditionally, deploying such an application requires a series of complex steps, including setting up a deployment, a service, an ingress, a horizontal pod autoscaler, a service account, and optionally, a certificate. Each of these steps involves intricate configurations and a deep understanding of Kubernetes' workings.

However, this Kubernetes operator simplifies the entire process by serving as an efficient abstraction layer over the Kubernetes API. It eliminates the need for manually creating each of the aforementioned resources. Instead, users only need to define a custom resource. Upon the creation of this custom resource, the Kubernetes operator automatically undertakes the task of generating all the necessary Kubernetes resources.

The primary advantage of using this operator lies in its simplification and automation of the deployment process. It allows users, particularly those who may not be deeply versed in Kubernetes intricacies, to deploy single-container applications with ease and reliability. This streamlined process not only saves time and reduces the potential for human error but also ensures a consistent deployment experience.

Furthermore, the [pocketbase-backend](https://github.com/janlauber/one-click/tree/main/pocketbase) of the project will leverage this operator to deploy applications to Kubernetes. This integration signifies a shift towards a more efficient, less error-prone deployment methodology, emphasizing automation and ease of use. In essence, this Kubernetes operator is a transformative tool, designed to make Kubernetes more accessible and manageable, particularly for deployments involving single-container applications.

## Rollout CRD

```yaml
apiVersion: one-click.dev/v1alpha1
kind: Rollout
metadata:
  name: nginx
  namespace: test
spec:
  args: ["nginx", "-g", "daemon off;"]
  command: ["nginx"]
  rolloutStrategy: rollingUpdate # or "recreate"
  image:
    registry: "docker.io"
    repository: "nginx"
    tag: "latest"
    username: "test"
    password: "test3"
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    allowPrivilegeEscalation: false
    runAsNonRoot: true
    readOnlyRootFilesystem: true
    privileged: false
    capabilities:
      drop:
        - ALL
      add:
        - NET_BIND_SERVICE
  horizontalScale:
    minReplicas: 1
    maxReplicas: 3
    targetCPUUtilizationPercentage: 80
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"
  env:
    - name: "REFLEX_USERNAME"
      value: "admin"
    - name: DEBUG
      value: "true"
  secrets:
    - name: "REFLEX_PASSWORD"
      value: "admin"
    - name: "ANOTHER_SECRET"
      value: "122"
  volumes:
    - name: "data"
      mountPath: "/data"
      size: "2Gi"
      storageClass: "standard"
  interfaces:
    - name: "http"
      port: 80
    - name: "https"
      port: 443
      ingress:
        ingressClass: "nginx"
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          nginx.ingress.kubernetes.io/ssl-redirect: "false"
        rules:
          - host: "reflex.oneclickapps.dev"
            path: "/"
            tls: true
            tlsSecretName: "wildcard-tls-secret"
          - host: "reflex.oneclickapps.dev"
            path: "/test"
            tls: false
  cronjobs:
    - name: some-bash-job
      suspend: false
      image:
        password: ""
        registry: docker.io
        repository: library/busybox
        tag: latest
        username: ""
      schedule: "*/1 * * * *"
      command: ["echo", "hello"]
      maxRetries: 3
      backoffLimit: 2
      env:
        - name: SOME_ENV
          value: "some-value"
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 100m
          memory: 256Mi
  serviceAccountName: "nginx"
```

## Build

```bash
make build
```

## Run

```bash
make install run # also runs the operator locally
```

## Deploy

```bash
make deploy IMG=<image> # deploy to cluster
```

## uninstall

```bash
make uninstall # uninstall from cluster
```
