package main

func (f *filter) defaultTemplates() map[string]string {
	return map[string]string{
		"cm":      cmTemplate,
		"sts":     stsTemplate,
		"svc":     svcTemplate,
		"dns-svc": dnsSvcTemplate,
		"ui-svc":  uiSvcTemplate,
	}
}

func (f *filter) gossipTemplates() map[string]string {
	return map[string]string{
		"gossip-job":         gossipJobTemplate,
		"gossip-sa":          gossipSATemplate,
		"gossip-role":        gossipRoleTemplate,
		"gossip-rolebinding": gossipRoleBindingTemplate,
	}
}

func (f *filter) tlsTemplates() map[string]string {
	return map[string]string{
		"tls-job":         tlsJobTemplate,
		"tls-sa":          tlsSATemplate,
		"tls-role":        tlsRoleTemplate,
		"tls-rolebinding": tlsRoleBindingTemplate,
		"tls-sts-patch":   tlsSTSPatchTemplate,
		"tls-cm-patch":    tlsCMPatchTemplate,
	}
}

var cmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
data:
  00-defaults.hcl: |-
    acl = {
      enabled = true
      default_policy = "allow"
      enable_token_persistence = true
    }

    connect = {
      enabled = true
    }
`

var stsTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
spec:
  serviceName: {{ .Name }}
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  template:
    spec:
      terminationGracePeriodSeconds: 10
      securityContext:
        fsGroup: 1000
      containers:
        - name: consul
          image: docker.io/library/consul:1.6.2
          command:
            - consul
            - agent
            - -advertise=$(POD_IP)
            - -bind=0.0.0.0
            - -bootstrap-expect=$(CONSUL_REPLICAS)
            - -client=0.0.0.0
            - -config-dir=/consul/config
            - -data-dir=/consul/data
            - -ui
            - -retry-join={{ .Name }}.$(NAMESPACE).svc.cluster.local
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
              value: "{{ .Replicas }}"
          volumeMounts:
            - name: data
              mountPath: /consul/data
            - name: configs
              mountPath: /consul/config
          lifecycle:
            preStop:
              exec:
                command:
                - /bin/sh
                - -c
                - consul leave
          ports:
            - containerPort: 8500
              name: http
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
                - "/bin/sh"
                - "-ec"
                - |
                  curl http://127.0.0.1:8500/v1/status/leader 2>/dev/null | \
                  grep -E '".+"'
            failureThreshold: 2
            initialDelaySeconds: 5
            periodSeconds: 3
            successThreshold: 1
            timeoutSeconds: 5
      volumes:
        - name: data
          emptyDir: {}
        - name: configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}
`

var svcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  ports:
    - name: http
      port: 8500
      targetPort: http
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

var dnsSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-dns
spec:
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

var uiSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-ui
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8500
`

var gossipJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-gossip-encryption
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-gossip-encryption
      restartPolicy: OnFailure
      initContainers:
        - name: generate-gossip-encryption-config
          image: docker.io/library/consul:1.6.2
          command:
            - /bin/sh
            - -ec
            - |-
              config_file=/config/generated/01-gossip-encryption.json
              cat <<EOF > "${config_file}"
              {
                "encrypt": "$(consul keygen)",
                "encrypt_verify_incoming": true,
                "encrypt_verify_outgoing": true
              }
          volumeMounts:
            - mountPath: /config/generated
              name: config-generated
      containers:
        - name: create-gossip-encryption-config-secret
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              secret="{{ .Name }}-gossip-encryption"
              config_dir="/config/generated"

              if [ ! "$(kubectl get secret --no-headers "${secret}"|wc -l)" = "0" ]; then
                echo "[INFO] \"secret/${secret}\" already exists. Exiting."
                exit 0
              fi

              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "--from-file=${config_dir}" "${secret}"
          volumeMounts:
            - mountPath: /config/generated
              name: config-generated
      volumes:
        - name: config-generated
          emptyDir: {}
`

var gossipSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-gossip-encryption
`

var gossipRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-gossip-encryption
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
      - create
`

var gossipRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-gossip-encryption
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-gossip-encryption
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-gossip-encryption
`

// A patch to configure the StatefulSet to use agent TLS encryption.
var tlsSTSPatchTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
spec:
  template:
    spec:
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
          env:
            - name: CONSUL_HTTP_ADDR
              value: https://127.0.0.1:8501
            - name: CONSUL_CACERT
              value: /consul/tls/consul-agent-ca.pem
            - name: CONSUL_CLIENT_CERT
              value: /consul/tls/dc1-cli-consul-0.pem
            - name: CONSUL_CLIENT_KEY
              value: /consul/tls/dc1-cli-consul-0-key.pem
          ports:
            - containerPort: 8501
              name: http
              protocol: "TCP"
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
          volumeMounts:
            - name: tls
              mountPath: /consul/tls
      volumes:
        - name: tls-secret
          secret:
            secretName: {{ .Name }}-agent-tls
        - name: tls
          emptyDir: {}
`

var tlsCMPatchTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
data:
  00-default-agent-tls.json: |-
    {
      "verify_incoming": true,
      "verify_outgoing": true,
      "verify_server_hostname": true,
      "auto_encrypt": {
        "allow_tls": true
      },
      "ca_file": "/consul/tls/consul-agent-ca.pem",
      "cert_file": "/consul/tls/server-consul.pem",
      "key_file": "/consul/tls/server-consul-key.pem",
      "ports": {
        "http": -1,
        "https": 8501
      }
    }
`

var tlsJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-agent-tls
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-agent-tls
      restartPolicy: OnFailure
      initContainers:
        - name: generate-agent-tls
          image: docker.io/library/consul:1.6.2
          command:
            - /bin/sh
            - -ec
            - |-
              tls_dir=/tls/generated
              cd "${tls_dir}"
              consul tls ca create
              consul tls cert create -cli
              consul tls cert create -server
              consul tls cert create -server
              consul tls cert create -server
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      containers:
        - name: create-agent-tls-secret
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              tls_dir="/tls/generated"
              prefix="{{ .Name }}-agent-tls"

              secret="${prefix}"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" "--from-file=${tls_dir}"

              secret="${prefix}-client-ca"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/consul-agent-ca.pem"

              secret="${prefix}-cli"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/dc1-cli-consul-0.pem" \
                "--from-file=${tls_dir}/dc1-cli-consul-0-key.pem"
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      volumes:
        - name: tls-generated
          emptyDir: {}
`

var tlsSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-agent-tls
`

var tlsRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-agent-tls
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
      - create
`

var tlsRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-agent-tls
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-agent-tls
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-agent-tls
`
