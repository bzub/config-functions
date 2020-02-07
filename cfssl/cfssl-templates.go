package cfssl

func cfsslJobTemplates() map[string]string {
	return map[string]string{
		"cfssl-job":         cfsslJobTemplate,
		"cfssl-sa":          cfsslSATemplate,
		"cfssl-role":        cfsslRoleTemplate,
		"cfssl-rolebinding": cfsslRoleBindingTemplate,
	}
}

var cfsslJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}
      restartPolicy: OnFailure
      initContainers:
        - name: cfssl
          image: docker.io/jitesoft/cfssl:828c23c
          command:
            - /bin/sh
            - -ec
            - |-
              cp /cfssl/configs/*.json /cfssl/certs
              cd /cfssl/certs
              cfssl gencert -initca ca_csr.json | cfssljson -bare ca -

              for csr in *_csr.json; do
                [ "${csr}" = "ca_csr.json" ] && continue
                instance="$(echo "${csr}"|cut -d"_" -f 1)"
                profile="$(echo "${csr}" |cut -d"_" -f 2)"

                cfssl gencert \
                  -ca=ca.pem \
                  -ca-key=ca-key.pem \
                  -config=config.json \
                  "-profile=${profile}" \
                  "${csr}" |\
                cfssljson -bare "${instance}-${profile}"
              done
              rm *.json
          envFrom:
            - configMapRef:
                name: {{ .Name }}
          volumeMounts:
            - mountPath: /cfssl/configs
              name: cfssl-configs
            - mountPath: /cfssl/certs
              name: cfssl-certs
      containers:
        - name: create-secret
          image: k8s.gcr.io/hyperkube:v1.17.2
          command:
            - /bin/sh
            - -ec
            - |-
              secret_dir="/cfssl/certs"
              secret_name="$(secret_name)"

              kubectl create secret generic \
                "--from-file=${secret_dir}" "${secret_name}"
          envFrom:
            - configMapRef:
                name: {{ .Name }}
          volumeMounts:
            - mountPath: /cfssl/certs
              name: cfssl-certs
      volumes:
        - name: cfssl-certs
        - name: cfssl-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}
`

var cfsslSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var cfsslRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}
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

var cfsslRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}
`
