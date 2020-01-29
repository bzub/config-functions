# Consul Configuration Function Agent Sidecar Injector Example

In this example we set up a production-grade Consul deployment and use the
agent sidecar injector feature on a workload Resource.

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/function-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-consul
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.3
data:
  agent_tls_enabled: "true"
  gossip_enabled: "true"
  acl_bootstrap_enabled: "true"
  agent_sidecar_injector_enabled: "true"
EOF
```

Now we create an example Deployment Resource that will have a Consul agent
sidecar added to it.

<!-- @createDeploymentForSidecar @test -->
```sh
cat <<EOF >$DEMO/my-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: other-namespace
  labels:
    app.kubernetes.io/instance: my-deployment
  annotations:
    config.bzub.dev/consul-agent-sidecar-injector: |-
      metadata:
        name: my-consul
        namespace: example
spec:
  selector:
    matchLabels:
      app.kubernetes.io/instance: my-deployment
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: my-deployment
    spec:
      containers:
        - name: example
          image: k8s.gcr.io/pause:3.1
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
config run $DEMO --global-scope
```

The `my-deployment` Deployment now has a Consul agent sidecar container,
configured with full TLS communication just like the server it targets.
<!-- @verifyDeployment @test -->
```sh
EXPECTED='.
└── [my-deployment.yaml]  Deployment other-namespace/my-deployment
    └── spec.template.spec.containers
        ├── 0
        │   └── name: consul-agent
        └── 1
            └── name: example'

TEST="$(config grep "kind=Deployment" $DEMO|config tree --name)"
[ "$TEST" = "$EXPECTED" ]
```

**NOTE**: Since full TLS communication is enabled, the sidecar will look for
Secrets with the following name formats:
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-tls-ca`
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-tls-client`
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-tls-cli`
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-gossip`

Also the following ConfigMaps:
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-agent`
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-client-tls`

These Secrets and ConfigMaps are automatically created in the Consul server's
namespace.  You will need to manually copy these Secrets to any additional
namespaces where sidecars will run.

For this example you can use kubectl/grep/sed to copy the Secrets from the
`example` namespace to the `other-namespace` namespace.

> ```sh
> kubectl -n example get cm -o yaml my-consul-example-agent |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> 
> kubectl -n example get cm -o yaml my-consul-example-client-tls |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> 
> kubectl -n example get secret -o yaml my-consul-example-tls-ca |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> 
> kubectl -n example get secret -o yaml my-consul-example-tls-client |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> 
> kubectl -n example get secret -o yaml my-consul-example-tls-cli |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> 
> kubectl -n example get secret -o yaml my-consul-example-gossip |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> ```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
