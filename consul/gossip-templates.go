package main

func (f *filter) gossipTemplates() map[string]string {
	return map[string]string{
		"gossip-job-cm":      gossipJobEnvTemplate,
		"gossip-job":         gossipJobTemplate,
		"gossip-sa":          gossipSATemplate,
		"gossip-role":        gossipRoleTemplate,
		"gossip-rolebinding": gossipRoleBindingTemplate,
	}
}

var gossipSecretVolumeTemplate = `
- secret:
    name: {{ .GossipSecretName }}
`

var gossipJobEnvTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-gossip-encryption-env
data:
  CONSUL_GOSSIP_SECRET: {{ .GossipSecretName }}
`

// CONSUL_GOSSIP_SECRET: {{ .Name }}-{{ .Namespace }}-gossip

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
              secret="$(CONSUL_GOSSIP_SECRET)"
              config_dir="/config/generated"
              kubectl create secret generic "--from-file=${config_dir}" "${secret}"
          envFrom:
            - configMapRef:
                name: {{ .Name }}-gossip-encryption-env
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
