package nodeexporter

func serverTemplates() map[string]string {
	return map[string]string{
		"server-ds":  serverDSTemplate,
		"server-svc": serverSvcTemplate,
	}
}

var serverDSTemplate = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
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
      hostNetwork: true
      hostPID: true
      nodeSelector:
        kubernetes.io/os: linux
      containers:
        - name: node-exporter
          image: quay.io/prometheus/node-exporter:v0.18.1
          args:
            - --path.procfs=/host/proc
            - --path.sysfs=/host/sys
            - --path.rootfs=/host/root
            - --no-collector.wifi
            - --no-collector.hwmon
            - --collector.filesystem.ignored-mount-points=^/(dev|proc|sys|var/lib/docker/.+)($|/)
            - --collector.filesystem.ignored-fs-types=^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|mqueue|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs)$
          ports:
            - name: http
              protocol: TCP
              containerPort: 9100
          resources:
            limits:
              cpu: 250m
              memory: 180Mi
            requests:
              cpu: 102m
              memory: 180Mi
          volumeMounts:
            - name: proc
              readOnly: false
              mountPath: /host/proc
            - name: sys
              readOnly: false
              mountPath: /host/sys
            - name: root
              readOnly: true
              mountPath: /host/root
              mountPropagation: HostToContainer
      volumes:
        - name: proc
          hostPath:
            path: /proc
        - name: sys
          hostPath:
            path: /sys
        - name: root
          hostPath:
            path: /
      tolerations:
      - operator: Exists
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
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
  ports:
    - name: http
      port: 9100
      targetPort: http
`
