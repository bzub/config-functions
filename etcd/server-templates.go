package etcd

func serverTemplates() map[string]string {
	return map[string]string{
		"server-cm":  serverCmTemplate,
		"server-sts": serverStsTemplate,
		"server-svc": serverSvcTemplate,
	}
}

var serverCmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  ETCD_DATA_DIR: /etcd/data
  ETCD_LOGGER: zap
  ETCD_INITIAL_CLUSTER_STATE: new
  ETCD_INITIAL_CLUSTER_TOKEN: default
  ETCD_INITIAL_CLUSTER: {{ .InitialCluster }}
  ETCD_LISTEN_CLIENT_URLS: https://0.0.0.0:2379
  ETCD_LISTEN_METRICS_URLS: http://0.0.0.0:8080
  ETCD_LISTEN_PEER_URLS: https://0.0.0.0:2380
  ETCD_CLIENT_CERT_AUTH: "true"
  ETCD_PEER_CLIENT_CERT_AUTH: "true"
  ETCD_TRUSTED_CA_FILE: /etcd/tls/ca.pem
  ETCD_CERT_FILE: /etcd/tls/server.pem
  ETCD_KEY_FILE: /etcd/tls/server-key.pem
  ETCD_PEER_TRUSTED_CA_FILE: /etcd/tls/ca.pem
  ETCD_PEER_CERT_FILE: /etcd/tls/peer.pem
  ETCD_PEER_KEY_FILE: /etcd/tls/peer-key.pem
  ETCDCTL_CACERT: /etcd/tls/ca.pem
  ETCDCTL_CERT: /etcd/tls/root-client.pem
  ETCDCTL_KEY: /etcd/tls/root-client-key.pem
`

var serverStsTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  serviceName: {{ .Name }}-server
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
      app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
        app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
    spec:
      initContainers:
        - name: etcd-server-tls-setup
          image: docker.io/library/alpine:3.11
          command:
            - /bin/sh
            - -ec
            - |-
              index="$(hostname|sed 's/.*-\(.*$\)/\1/')"
              cp "/etcd/tls/secret/ca.pem" /etcd/tls
              cp "/etcd/tls/secret/${index}-server.pem" /etcd/tls/server.pem
              cp "/etcd/tls/secret/${index}-server-key.pem" /etcd/tls/server-key.pem
              cp "/etcd/tls/secret/${index}-peer.pem" /etcd/tls/peer.pem
              cp "/etcd/tls/secret/${index}-peer-key.pem" /etcd/tls/peer-key.pem
              cp "/etcd/tls/secret/root-client.pem" /etcd/tls
              cp "/etcd/tls/secret/root-client-key.pem" /etcd/tls
          volumeMounts:
            - name: etcd-server-tls-secret
              mountPath: /etcd/tls/secret
            - name: etcd-server-tls
              mountPath: /etcd/tls
      containers:
        - name: etcd-server
          image: gcr.io/etcd-development/etcd:v3.4.3
          command:
            - /usr/local/bin/etcd
            - --name=$(HOSTNAME)
            - --advertise-client-urls=https://$(HOSTNAME):2379
            - --initial-advertise-peer-urls=https://$(HOSTNAME):2380
          envFrom:
            - configMapRef:
                name: {{ .Name }}
            - configMapRef:
                name: {{ .Name }}-server
          env:
            - name: HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
          ports:
            - containerPort: 2379
              name: client
            - containerPort: 2380
              name: peer
            - containerPort: 8080
              name: metrics
          readinessProbe:
            exec:
              command:
                - etcdctl
                - endpoint
                - health
          volumeMounts:
            - name: etcd-server-data
              mountPath: /etcd/data
            - name: etcd-server-tls
              mountPath: /etcd/tls
              readOnly: true
      volumes:
        - name: etcd-server-data
        - name: etcd-server-tls
        - name: etcd-server-tls-secret
          projected:
            sources:
              - secret:
                  name: {{ .Data.TLSCASecretName }}
              - secret:
                  name: {{ .Data.TLSServerSecretName }}
              - secret:
                  name: {{ .Data.TLSRootClientSecretName }}
`

var serverSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  annotations:
    config.bzub.dev/prometheus-scrape_configs: |-
      - job_name: {{ .Name }}
        kubernetes_sd_configs:
          - role: endpoints
            namespaces:
              names:
                - {{ .Namespace }}
        relabel_configs:
          - source_labels: [__meta_kubernetes_service_name]
            action: keep
            regex: {{ .Name }}-server
          - source_labels: [__meta_kubernetes_endpoint_port_name]
            action: keep
            regex: metrics
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
spec:
  selector:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  clusterIP: None
  publishNotReadyAddresses: true
  ports:
    - name: etcd-server-ssl
      port: 2380
    - name: etcd-client-ssl
      port: 2379
    - name: metrics
      port: 8080
`
