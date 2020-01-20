package main

func (f *filter) unsealJobTemplates() map[string]string {
	return map[string]string{
		"unseal-job":         unsealJobTemplate,
		"unseal-sa":          unsealSATemplate,
		"unseal-role":        unsealRoleTemplate,
		"unseal-rolebinding": unsealRoleBindingTemplate,
	}
}

var unsealJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-unseal
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-unseal
      restartPolicy: OnFailure
      containers:
        - name: vault-unseal
          image: k8s.gcr.io/hyperkube:v1.17.1
          command:
            - /bin/sh
            - -ec
            - |-
              secrets_dir="/vault/secrets"

              # TODO: Support multiple keys
              unseal_key_b64="$(\
                cat /vault/secrets/init.json |\
                  jq -r '.unseal_keys_b64[0]' \
              )"

              vault_server_pods="$(\
                kubectl get pods -o name \
                -l "app.kubernetes.io/name=vault-server" \
                -l "app.kubernetes.io/instance={{ .Name }}" \
              )"

              for pod in ${vault_server_pods}; do
                kubectl exec "${pod}" -c vault-server -- \
                  vault operator unseal "${unseal_key_b64}"
              done
          envFrom:
            - configMapRef:
                name: my-vault
          volumeMounts:
            - name: vault-secrets
              mountPath: /vault/secrets
      volumes:
        - name: vault-secrets
          projected:
            sources:
              - secret:
                  name: {{ .Data.UnsealSecretName }}
`

var unsealSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-unseal
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var unsealRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-unseal
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - pods/exec
    verbs:
      - create
`

var unsealRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-unseal
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-unseal
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-unseal
`
