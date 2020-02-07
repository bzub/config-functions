package prometheus

func serverTemplates() map[string]string {
	return map[string]string{
		"server-cm":          serverCmTemplate,
		"server-sts":         serverStsTemplate,
		"server-svc":         serverSvcTemplate,
		"server-sa":          serverSATemplate,
		"server-role":        serverRoleTemplate,
		"server-rolebinding": serverRoleBindingTemplate,
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
  prometheus.yml: |-
    global:
      scrape_interval:     15s
      evaluation_interval: 30s
    scrape_configs:
{{- range $i, $scrapeConfig := .Data.ScrapeConfigs }}
{{ $scrapeConfig }}
{{ end }}
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
      serviceAccountName: {{ .Name }}-server
      terminationGracePeriodSeconds: 600
      containers:
        - name: prometheus
          image: docker.io/prom/prometheus:v2.15.2
          args:
            - --config.file=/prometheus/config/prometheus.yml
            - --web.console.templates=/etc/prometheus/consoles
            - --web.console.libraries=/etc/prometheus/console_libraries
            - --storage.tsdb.path=/prometheus/data
            - --storage.tsdb.retention.time=24h
            - --storage.tsdb.no-lockfile
          ports:
            - name: web
              protocol: TCP
              containerPort: 9090
          volumeMounts:
            - name: data
              mountPath: /prometheus/data
            - name: config
              mountPath: /prometheus/config
          livenessProbe:
            httpGet:
              port: web
              path: /-/healthy
              scheme: HTTP
          readinessProbe:
            httpGet:
              port: web
              path: /-/ready
              scheme: HTTP
      volumes:
        - name: data
        - name: config
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}-server
      securityContext:
        fsGroup: 2000
        runAsNonRoot: true
        runAsUser: 1000
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
  sessionAffinity: ClientIP
  ports:
    - name: web
      port: 9090
      targetPort: web
`

var serverSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var serverRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
rules:
  - apiGroups:
      - ""
    resources:
      - services
      - endpoints
      - pods
      - configmaps
    verbs:
      - get
      - list
      - watch
`

var serverRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-server
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-server
`
