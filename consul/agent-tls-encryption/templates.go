package main

func (f *filter) defaultTemplates() map[string]string {
	return map[string]string{
		"job":            jobTemplate,
		"serviceaccount": serviceAccountTemplate,
		"role":           roleTemplate,
		"rolebinding":    roleBindingTemplate,
		"statefulset":    statefulSetPatchTemplate,
		"configmap":      configMapPatchTemplate,
	}
}

// A patch to configure the StatefulSet to use agent TLS encryption.
var statefulSetPatchTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .StatefulSetName }}
  annotations:
    cfunc/preserve-label/app.kubernetes.io/name: "true"
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
            secretName: {{ .Name }}
        - name: tls
          emptyDir: {}
`

var configMapPatchTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .ConfigMapName }}
  annotations:
    cfunc/preserve-label/app.kubernetes.io/name: "true"
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

var jobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}
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
              secret="{{ .Name }}"
              tls_dir="/tls/generated"

              if [ ! "$(kubectl get secret --no-headers "${secret}"|wc -l)" = "0" ]; then
                echo "[INFO] \"secret/${secret}\" already exists. Exiting."
                exit 0
              fi

              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "--from-file=${tls_dir}" "${secret}"
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      volumes:
        - name: tls-generated
          emptyDir: {}
`

var serviceAccountTemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}
`

var roleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}
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

var roleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}
`
