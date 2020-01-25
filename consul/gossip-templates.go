package consul

func (f *ConsulFilter) gossipTemplates() map[string]string {
	return map[string]string{
		"gossip-job":         gossipJobTemplate,
		"gossip-sa":          gossipSATemplate,
		"gossip-role":        gossipRoleTemplate,
		"gossip-rolebinding": gossipRoleBindingTemplate,
	}
}

var gossipJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-gossip-encryption
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-gossip-encryption
      restartPolicy: OnFailure
      initContainers:
        - name: generate-gossip-encryption-config
          image: docker.io/library/consul:1.7.0-beta2
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
          image: k8s.gcr.io/hyperkube:v1.17.1
          command:
            - /bin/sh
            - -ec
            - |-
              secret="$(gossip_secret_name)"
              config_dir="/config/generated"
              kubectl create secret generic "--from-file=${config_dir}" "${secret}"
          envFrom:
            - configMapRef:
                name: {{ .Name }}
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
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var gossipRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-gossip-encryption
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
      - create
`

var gossipRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-gossip-encryption
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-gossip-encryption
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-gossip-encryption
`
