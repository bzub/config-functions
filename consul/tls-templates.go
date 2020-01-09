package main

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
            secretName: {{ .Name }}-{{ .Namespace }}-agent-tls
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
              prefix="{{ .Name }}-{{ .Namespace }}-agent-tls"

              secret="${prefix}"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" "--from-file=${tls_dir}"

              secret="${prefix}-ca"
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

// RBAC
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
