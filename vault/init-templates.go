package main

func (f *filter) initJobTemplates() map[string]string {
	return map[string]string{
		"init-job":         initJobTemplate,
		"init-sa":          initSATemplate,
		"init-role":        initRoleTemplate,
		"init-rolebinding": initRoleBindingTemplate,
	}
}

var initJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-init
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-init
      restartPolicy: OnFailure
      containers:
        - name: create-unseal-secret
          image: k8s.gcr.io/hyperkube:v1.17.1
          command:
            - /bin/sh
            - -ec
            - |-
              secret_name="$(unseal_secret_name)"
              exec_pod="{{ .Name }}-server-0"

              init_json="$(\
                kubectl exec "${exec_pod}" -- \
                  vault operator init -format="json" -n 1 -t 1\
              )"

              kubectl create secret generic \
                "--from-literal=init.json=${init_json}" "${secret_name}"
          envFrom:
            - configMapRef:
                name: {{ .Name }}
`

var initSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-init
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var initRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-init
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

var initRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-init
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-init
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-init
`
