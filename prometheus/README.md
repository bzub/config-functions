[prometheus]: https://prometheus.io/
[FunctionConfig]: https://pkg.go.dev/github.com/bzub/config-functions/prometheus?tab=doc#FunctionConfig
[FunctionData]: https://pkg.go.dev/github.com/bzub/config-functions/prometheus?tab=doc#FunctionData

# Prometheus Configuration Function

Creates Resource configs to deploy [Prometheus][prometheus] on Kubernetes.

## Function Features

The function metadata is documented in the [FunctionConfig][FunctionConfig] Go
type. The options available to configure the function are documented in the
[FunctionData][FunctionData] type.

## Getting Started

In the following example we create Resource configs for a Prometheus server. These
configs are meant to be checked into version control, so Secrets are not
included. Optionally, all necessary Secrets can be created in-cluster via Jobs
-- check out the [production demo](./productionExample.md).

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/my-prometheus_configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-prometheus
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/prometheus:v0.0.1
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
config run $DEMO
```

## Generated Resources

The function generates the following resources.
<!-- @verifyResources @test -->
```sh
EXPECTED='.
├── [Resource]  ConfigMap example/my-prometheus-server
├── [Resource]  Role example/my-prometheus-server
├── [Resource]  RoleBinding example/my-prometheus-server
├── [Resource]  Service example/my-prometheus-server
├── [Resource]  ServiceAccount example/my-prometheus-server
├── [Resource]  StatefulSet example/my-prometheus-server
└── [Resource]  ConfigMap example/my-prometheus'

TEST="$(config tree $DEMO --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

## Configuration

### Default Function Configuration

The function adds any missing configuration fields to the function ConfigMap we
created above, populating their values with defaults.

<!-- @verifyFunctionConfigDefaults @test -->
```sh
EXPECTED='apiVersion: v1
kind: ConfigMap
metadata:
  name: my-prometheus
  namespace: "example"
  labels:
    app.kubernetes.io/instance: my-prometheus
    app.kubernetes.io/name: prometheus-server
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/prometheus:v0.0.1'

TEST="$(cat $DEMO/my-prometheus_configmap.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

### Adding Scrape Configs

Scrape configs can be added from annotations on the Resources inteded to be
scraped for metrics. This function looks for Resources in the same namespace
with the annotation `config.bzub.dev/prometheus-scrape_configs`.

<!-- @createScrapeConfigAnnotation @test -->
```sh
cat <<EOF >$DEMO/my-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: example
  annotations:
    config.bzub.dev/prometheus-scrape_configs: |-
      - job_name: my-service
        kubernetes_sd_configs:
          - role: endpoints
            namespaces:
              names:
                - example
        relabel_configs:
          - source_labels: [__meta_kubernetes_service_name]
            action: keep
            regex: my-service
          - source_labels: [__meta_kubernetes_endpoint_port_name]
            action: keep
            regex: metrics
spec:
  ports:
    - name: metrics
      port: 8080
EOF
```

Delete the old Prometheus server ConfigMap file and regenerate it.
<!-- @regenerateServerCMWithScrapeConfig @test -->
```sh
rm $DEMO/my-prometheus-server_configmap.yaml
config run $DEMO

EXPECTED='apiVersion: v1
kind: ConfigMap
metadata:
  name: my-prometheus-server
  namespace: "example"
  labels:
    app.kubernetes.io/instance: my-prometheus
    app.kubernetes.io/name: prometheus-server
data:
  prometheus.yml: |-
    global:
      scrape_interval:     15s
      evaluation_interval: 30s
    scrape_configs:
      - job_name: my-prometheus
        kubernetes_sd_configs:
          - role: endpoints
            namespaces:
              names:
                - example
        relabel_configs:
          - source_labels: [__meta_kubernetes_service_name]
            action: keep
            regex: my-prometheus-server
          - action: labelmap
            regex: __meta_kubernetes_service_label_(.+)
          - source_labels: [__meta_kubernetes_namespace]
            action: replace
            target_label: kubernetes_namespace
          - source_labels: [__meta_kubernetes_service_name]
            action: replace
            target_label: kubernetes_service_name
          - action: labelmap
            regex: __meta_kubernetes_pod_label_(.+)
          - source_labels: [__meta_kubernetes_pod_name]
            action: replace
            target_label: kubernetes_pod_name

      - job_name: my-service
        kubernetes_sd_configs:
          - role: endpoints
            namespaces:
              names:
                - example
        relabel_configs:
          - source_labels: [__meta_kubernetes_service_name]
            action: keep
            regex: my-service
          - source_labels: [__meta_kubernetes_endpoint_port_name]
            action: keep
            regex: metrics'

TEST="$(cat $DEMO/my-prometheus-server_configmap.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
