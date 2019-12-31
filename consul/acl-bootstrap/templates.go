package main

func (f *filter) defaultTemplates() map[string]string {
	return map[string]string{
		"job":            jobTemplate,
		"serviceaccount": serviceAccountTemplate,
		"role":           roleTemplate,
		"rolebinding":    roleBindingTemplate,
	}
}

var jobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}
spec:
  template:
    metadata:
    spec:
      serviceAccountName: {{ .Name }}
      restartPolicy: OnFailure
      containers:
        - name: consul-acl-bootstrap
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              consul_secret="{{ .Name }}"
              exec_arg="{{ .ExecArg }}"

              metadata_dir="/metadata/consul-secrets"
              if [ "$(ls "${metadata_dir}"|wc -l)" = "0" ]; then
                metadata_dir="/metadata/consul-init"

                echo "[INFO] Performing consul acl bootstrap."
                export consul_bootstrap_out="$(kubectl exec "${exec_arg}" -- consul acl bootstrap)"
                if [ "${consul_bootstrap_out}" = "" ]; then
                  echo "[ERROR] No consul acl bootstrap output. Is consul up and running?"
                  exit 1
                fi
                echo "${consul_bootstrap_out}"|grep AccessorID|awk '{print $2}'|tr -d '\n' > "${metadata_dir}/accessor_id.txt"
                echo "${consul_bootstrap_out}"|grep SecretID|awk '{print $2}'|tr -d '\n' > "${metadata_dir}/secret_id.txt"

                echo "[INFO] Creating \"secret/${consul_secret}\"."
                kubectl create secret generic "--from-file=${metadata_dir}" "${consul_secret}"
              fi

              export CONSUL_HTTP_TOKEN="$(cat "${metadata_dir}/secret_id.txt")"
              kubectl exec "${exec_arg}" -- /bin/sh -c "CONSUL_HTTP_TOKEN=$(cat "${metadata_dir}/secret_id.txt") consul acl token list"
          volumeMounts:
            - mountPath: /metadata/consul-init
              name: consul-init
            - mountPath: /metadata/consul-secrets
              name: consul-secrets
      volumes:
        - name: consul-init
          emptyDir: {}
        - name: consul-secrets
          secret:
            secretName: {{ .Name }}
            optional: true
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
  - apiGroups:
      - apps
    resources:
      - statefulsets
    verbs:
      - get
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
