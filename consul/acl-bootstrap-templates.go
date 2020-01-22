package consul

func (f *ConsulFilter) aclJobTemplates() map[string]string {
	return map[string]string{
		"acl-job":         aclJobTemplate,
		"acl-sa":          aclSATemplate,
		"acl-role":        aclRoleTemplate,
		"acl-rolebinding": aclRoleBindingTemplate,
	}
}

var aclJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-acl-bootstrap
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-acl-bootstrap
      restartPolicy: OnFailure
      containers:
        - name: consul-acl-bootstrap
          image: k8s.gcr.io/hyperkube:v1.17.1
          command:
            - /bin/sh
            - -ec
            - |-
              secret_dir="/consul/acl-bootstrap"
              secret_name="$(acl_bootstrap_secret_name)"
              exec_pod="{{ .Name }}-server-0"

              echo "[INFO] Performing consul acl bootstrap."
              output="$(kubectl exec "${exec_pod}" -- consul acl bootstrap)"

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
                name: {{ .Name }}
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
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var aclRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-acl-bootstrap
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
  - apiGroups:
      - ""
    resources:
      - pods/exec
    verbs:
      - create
    resourceNames:
      - {{ .Name }}-server-0
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
    resourceNames:
      - {{ .Name }}-server-0
`

var aclRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-acl-bootstrap
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-acl-bootstrap
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-acl-bootstrap
`
