package consul

func serverTemplates() map[string]string {
	return map[string]string{
		"agent-cm":       agentCmTemplate,
		"server-cm":      serverCmTemplate,
		"server-sts":     serverStsTemplate,
		"server-svc":     serverSvcTemplate,
		"server-dns-svc": serverDNSSvcTemplate,
		"server-ui-svc":  serverUISvcTemplate,
	}
}

var agentCmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-{{ .Namespace }}-agent
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  00-agent-defaults.hcl: |-
    datacenter = "dc1"
    data_dir = "/consul/data"
    verify_incoming = true
    verify_outgoing = true
    ca_file = "/consul/tls/consul-agent-ca.pem"
    ports = {
      http = -1
      https = 8500
    }
`

var serverCmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-{{ .Namespace }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  00-acl-defaults.hcl: |-
    acl = {
      enabled = true
      default_policy = "allow"
      enable_token_persistence = true
    }
  00-connect-defaults.hcl: |-
    connect = {
      enabled = true
    }
  00-server-defaults.hcl: |-
    verify_server_hostname = true
    cert_file = "/consul/tls/server-consul.pem"
    key_file = "/consul/tls/server-consul-key.pem"
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
  replicas: 1 # {"description":"Consul server replicas.","type":"integer","x-kustomize":{"setter":{"name":"{{ .Name }}-replicas","value":"1"}}}
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
      securityContext:
        fsGroup: 1000
      initContainers:
        - name: consul-server-tls-setup
          image: docker.io/library/alpine:3.11
          command:
            - /bin/sh
            - -ec
            - |-
              index="$(hostname|sed 's/.*-\(.*$\)/\1/')"
              cp /consul/tls/secret/consul-agent-ca.pem /consul/tls
              cp /consul/tls/secret/dc1-server-consul-${index}.pem \
                 /consul/tls/server-consul.pem
              cp /consul/tls/secret/dc1-server-consul-${index}-key.pem \
                 /consul/tls/server-consul-key.pem
              cp /consul/tls/secret/dc1-cli-consul-0.pem /consul/tls
              cp /consul/tls/secret/dc1-cli-consul-0-key.pem /consul/tls
          volumeMounts:
            - name: tls-secret
              mountPath: /consul/tls/secret
            - name: tls
              mountPath: /consul/tls
      containers:
        - name: consul
          image: docker.io/library/consul:1.7.1
          command:
            - consul
            - agent
            - -advertise=$(POD_IP)
            - -bind=0.0.0.0
            - -bootstrap-expect=$(CONSUL_REPLICAS)
            - -client=0.0.0.0
            - -config-dir=/consul/config
            - -ui
            - -retry-join={{ .Name }}-server.$(NAMESPACE).svc.cluster.local
            - -server
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: CONSUL_REPLICAS
              value: "1" # {"description":"Consul server replicas.","type":"string","x-kustomize":{"setter":{"name":"{{ .Name }}-replicas","value":"1"}}}
            - name: CONSUL_HTTP_ADDR
              value: https://127.0.0.1:8500
            - name: CONSUL_CACERT
              value: /consul/tls/consul-agent-ca.pem
            - name: CONSUL_CLIENT_CERT
              value: /consul/tls/dc1-cli-consul-0.pem
            - name: CONSUL_CLIENT_KEY
              value: /consul/tls/dc1-cli-consul-0-key.pem
          volumeMounts:
            - name: consul-data
              mountPath: /consul/data
            - name: consul-configs
              mountPath: /consul/config
            - name: tls
              mountPath: /consul/tls
          lifecycle:
            preStop:
              exec:
                command:
                - /bin/sh
                - -c
                - consul leave
          ports:
            - containerPort: 8500
              name: https
              protocol: "TCP"
            - containerPort: 8301
              name: serflan-tcp
              protocol: "TCP"
            - containerPort: 8301
              name: serflan-udp
              protocol: "UDP"
            - containerPort: 8302
              name: serfwan-tcp
              protocol: "TCP"
            - containerPort: 8302
              name: serfwan-udp
              protocol: "UDP"
            - containerPort: 8300
              name: server
              protocol: "TCP"
            - containerPort: 8600
              name: dns-tcp
              protocol: "TCP"
            - containerPort: 8600
              name: dns-udp
              protocol: "UDP"
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -ec
                - |
                  curl \
                    --cacert $(CONSUL_CACERT) \
                    --cert $(CONSUL_CLIENT_CERT) \
                    --key $(CONSUL_CLIENT_KEY) \
                    $(CONSUL_HTTP_ADDR)/v1/status/leader 2>/dev/null |\
                  grep -E '".+"'
      volumes:
        - name: consul-data
          emptyDir: {}
        - name: consul-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}-{{ .Namespace }}-agent
              - configMap:
                  name: {{ .Name }}-{{ .Namespace }}-server
              - secret:
                  name: {{ .Data.GossipSecretName }}
        - name: tls-secret
          secret:
            secretName: {{ .Data.TLSServerSecretName }}
        - name: tls
          emptyDir: {}
`

var serverSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  selector:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  clusterIP: None
  publishNotReadyAddresses: true
  ports:
    - name: https
      port: 8500
      targetPort: https
    - name: serflan-tcp
      protocol: "TCP"
      port: 8301
      targetPort: serflan-tcp
    - name: serflan-udp
      protocol: "UDP"
      port: 8301
      targetPort: serflan-udp
    - name: serfwan-tcp
      protocol: "TCP"
      port: 8302
      targetPort: serfwan-tcp
    - name: serfwan-udp
      protocol: "UDP"
      port: 8302
      targetPort: serfwan-udp
    - name: server
      port: 8300
      targetPort: server
    - name: dns-tcp
      protocol: "TCP"
      port: 8600
      targetPort: dns-tcp
    - name: dns-udp
      protocol: "UDP"
      port: 8600
      targetPort: dns-udp
`

var serverDNSSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-server-dns
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  selector:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  ports:
    - name: dns-tcp
      port: 53
      protocol: "TCP"
      targetPort: dns-tcp
    - name: dns-udp
      port: 53
      protocol: "UDP"
      targetPort: dns-udp
`

var serverUISvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-server-ui
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  selector:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  ports:
    - name: https
      port: 443
      targetPort: 8500
`
