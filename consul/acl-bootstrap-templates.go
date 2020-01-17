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
              secret_dir="/consul/acl-bootstrap"
              secret_name="$(CONSUL_ACL_BOOTSTRAP_SECRET)"
              exec_arg="sts/{{ .Name }}"

              echo "[INFO] Performing consul acl bootstrap."
              output="$(kubectl exec "${exec_arg}" -- consul acl bootstrap)"

              if [ "${output}" = "" ]; then
                echo "[ERROR] No consul acl bootstrap output. Is consul up and running?"
                exit 1
              fi

              echo "${output}"|grep AccessorID|awk '{print $2}'|tr -d '\n' >\
                "${secret_dir}/accessor_id.txt"
              echo "${output}"|grep SecretID|awk '{print $2}'|tr -d '\n' >\
                "${secret_dir}/secret_id.txt"

              kubectl create secret generic \
                "--from-file=${secret_dir}" "${secret_name}"
          envFrom:
            - configMapRef:
                name: {{ .Name }}-acl-bootstrap-env
          volumeMounts:
            - mountPath: /consul/acl-bootstrap
              name: consul-init
      volumes:
        - name: consul-init
          emptyDir: {}
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
