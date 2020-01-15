package main

func (f *filter) aclJobTemplates() map[string]string {
	return map[string]string{
		"acl-job-cm":      aclJobEnvTemplate,
		"acl-job":         aclJobTemplate,
		"acl-sa":          aclSATemplate,
		"acl-role":        aclRoleTemplate,
		"acl-rolebinding": aclRoleBindingTemplate,
	}
}

var aclJobEnvTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-acl-bootstrap-env
data:
  CONSUL_ACL_BOOTSTRAP_SECRET: {{ .Name }}-{{ .Namespace }}-acl
`

var aclJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-acl-bootstrap
spec:
  template:
    metadata:
    spec:
      serviceAccountName: {{ .Name }}-acl-bootstrap
      restartPolicy: OnFailure
      containers:
        - name: consul-acl-bootstrap
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              consul_secret="$(CONSUL_ACL_BOOTSTRAP_SECRET)"
              exec_arg="sts/{{ .Name }}"

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

                kubectl create secret generic "--from-file=${metadata_dir}" "${consul_secret}"
              fi

              export CONSUL_HTTP_TOKEN="$(cat "${metadata_dir}/secret_id.txt")"
              kubectl exec "${exec_arg}" -- /bin/sh -c "CONSUL_HTTP_TOKEN=$(cat "${metadata_dir}/secret_id.txt") consul acl token list"
          envFrom:
            - configMapRef:
                name: {{ .Name }}-acl-bootstrap-env
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
            secretName: {{ .Name }}-acl-bootstrap
            optional: true
`

var aclSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-acl-bootstrap
`

var aclRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-acl-bootstrap
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

var aclRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-acl-bootstrap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-acl-bootstrap
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-acl-bootstrap
`
