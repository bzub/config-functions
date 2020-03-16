package consul

func tlsTemplates() map[string]string {
	return map[string]string{
		"tls-job":         tlsJobTemplate,
		"tls-sa":          tlsSATemplate,
		"tls-role":        tlsRoleTemplate,
		"tls-rolebinding": tlsRoleBindingTemplate,
	}
}

var tlsJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-tls
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-tls
      restartPolicy: OnFailure
      initContainers:
        - name: generate-tls
          image: docker.io/library/consul:1.7.1
          command:
            - /bin/sh
            - -ec
            - |-
              tls_dir=/tls/generated
              cd "${tls_dir}"
              consul tls ca create
              consul tls cert create -cli
              consul tls cert create -client
              for i in $(seq 3); do
                consul tls cert create -server \
                  -additional-dnsname "{{ .Name }}-server.{{ .Namespace }}" \
                  -additional-dnsname "{{ .Name }}-server.{{ .Namespace }}.svc"
              done
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      containers:
        - name: create-tls-secret
          image: k8s.gcr.io/hyperkube:v1.17.4
          command:
            - /bin/sh
            - -ec
            - |-
              tls_dir="/tls/generated"

              secret="$(tls_server_secret_name)"
              kubectl create secret generic "${secret}" "--from-file=${tls_dir}"

              secret="$(tls_ca_secret_name)"
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/consul-agent-ca.pem"

              secret="$(tls_cli_secret_name)"
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/dc1-cli-consul-0.pem" \
                "--from-file=${tls_dir}/dc1-cli-consul-0-key.pem"

              secret="$(tls_client_secret_name)"
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/dc1-client-consul-0.pem" \
                "--from-file=${tls_dir}/dc1-client-consul-0-key.pem"
          envFrom:
            - configMapRef:
                name: {{ .Name }}
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
  name: {{ .Name }}-tls
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var tlsRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-tls
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
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
  name: {{ .Name }}-tls
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-tls
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-tls
`
