package consul

func backupCronJobTemplates() map[string]string {
	return map[string]string{
		"backup-cronjob":              backupCronJobTemplate,
		"backup-sa":                   backupSATemplate,
		"backup-role":                 backupRoleTemplate,
		"backup-rolebinding":          backupRoleBindingTemplate,
		"restore-secrets-cronjob":     restoreSecretsCronJobTemplate,
		"restore-secrets-sa":          restoreSecretsSATemplate,
		"restore-secrets-role":        restoreSecretsRoleTemplate,
		"restore-secrets-rolebinding": restoreSecretsRoleBindingTemplate,
		"restore-snapshot-cronjob":    restoreSnapshotCronJobTemplate,
	}
}

var backupCronJobTemplate = `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: {{ .Name }}-backup
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  schedule: 0 */1 * * *
  jobTemplate:
    metadata:
      labels:
        app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
        app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
    spec:
      template:
        metadata:
          name: consul-snaphot-save
          labels:
            app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
            app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
        spec:
          serviceAccountName: {{ .Name }}-backup
          restartPolicy: OnFailure
          initContainers:
            - name: consul-snapshot-save
              image: docker.io/library/consul:1.7.2
              command:
                - consul
                - snapshot
                - save
                - /consulbackup/backup.snap
              env:
                - name: CONSUL_HTTP_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: {{ .Data.ACLBootstrapSecretName }}
                      key: secret_id.txt
                      optional: true
{{- if .Data.TLSGeneratorJobEnabled }}
                - name: CONSUL_HTTP_ADDR
                  value: https://{{ .Name }}-server.{{ .Namespace }}.svc:8500
                - name: CONSUL_CACERT
                  value: /consul/tls/consul-agent-ca.pem
                - name: CONSUL_CLIENT_CERT
                  value: /consul/tls/dc1-cli-consul-0.pem
                - name: CONSUL_CLIENT_KEY
                  value: /consul/tls/dc1-cli-consul-0-key.pem
{{- else }}
                - name: CONSUL_HTTP_ADDR
                  value: http://{{ .Name }}-server.{{ .Namespace }}.svc:8500
{{- end }}
              volumeMounts:
                - name: consul-backup
                  mountPath: /consulbackup
{{- if .Data.TLSGeneratorJobEnabled }}
                - name: consul-tls-secret
                  mountPath: /consul/tls
{{- end }}
          containers:
            - name: consul-backup-ship
              image: k8s.gcr.io/hyperkube:v1.17.4
              command:
                - /bin/sh
                - -ec
                - |-
                  mkdir /k8s-backup
                  if [ "${acl_bootstrap_job_enabled}" = "true" ]; then
                    kubectl get secret "${acl_bootstrap_secret_name}" -o yaml |\
                      grep -Ev 'creationTimestamp|resourceVersion|selfLink|uid' \
                      > "/k8s-backup/${acl_bootstrap_secret_name}.yaml"
                  fi

                  if [ "${tls_generator_job_enabled}" = "true" ]; then
                    for secret in "${tls_server_secret_name}" "${tls_ca_secret_name}" "${tls_cli_secret_name}" "${tls_client_secret_name}"; do
                      kubectl get secret "${secret}" -o yaml |\
                        grep -Ev 'creationTimestamp|resourceVersion|selfLink|uid' \
                        > "/k8s-backup/${secret}.yaml"
                      done
                  fi

                  if [ "${gossip_key_generator_job_enabled}" = "true" ]; then
                    kubectl get secret "${gossip_secret_name}" -o yaml |\
                      grep -Ev 'creationTimestamp|resourceVersion|selfLink|uid' \
                      > "/k8s-backup/${gossip_secret_name}.yaml"
                  fi

                  kubectl create secret generic "${backup_secret_name}" \
                    --from-file=/k8s-backup \
                    --from-file=/consulbackup
              envFrom:
                - configMapRef:
                    name: {{ .Name }}
              volumeMounts:
                - name: consul-backup
                  mountPath: /consulbackup
          volumes:
            - name: consul-backup
{{- if .Data.TLSGeneratorJobEnabled }}
            - name: consul-tls-secret
              projected:
                sources:
                  - secret:
                      name: {{ .Data.TLSCASecretName }}
                  - secret:
                      name: {{ .Data.TLSCLISecretName }}
                  - secret:
                      name: {{ .Data.TLSClientSecretName }}
{{- end }}
`

var backupSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-backup
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var backupRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-backup
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
      - secrets
    verbs:
      - get
    resourceNames:
      - {{ .Data.BackupSecretName }}
      - {{ .Data.ACLBootstrapSecretName }}
      - {{ .Data.GossipSecretName }}
      - {{ .Data.TLSCASecretName }}
      - {{ .Data.TLSCLISecretName }}
      - {{ .Data.TLSClientSecretName }}
      - {{ .Data.TLSServerSecretName }}
`

var backupRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-backup
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-backup
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-backup
`

// Restore Secrets Resources
var restoreSecretsCronJobTemplate = `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: {{ .Name }}-restore-secrets
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  schedule: 0 */1 * * *
  suspend: true
  jobTemplate:
    metadata:
      labels:
        app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
        app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
    spec:
      template:
        metadata:
          name: consul-restore-secrets
          labels:
            app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
            app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
        spec:
          serviceAccountName: {{ .Name }}-restore-secrets
          restartPolicy: OnFailure
          containers:
            - name: consul-restore-secrets
              image: k8s.gcr.io/hyperkube:v1.17.4
              command:
                - /bin/sh
                - -ec
                - |-
                  for secret_yaml in /consul/restore/*.yaml; do
                    kubectl create -f "${secret_yaml}"
                  done
              volumeMounts:
                - name: restore-secret
                  mountPath: /consul/restore
          volumes:
            - name: restore-secret
              projected:
                sources:
                  - secret:
                      name: {{ .Data.RestoreSecretName }}
`

var restoreSecretsSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-restore-secrets
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
`

var restoreSecretsRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-restore-secrets
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

var restoreSecretsRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-restore-secrets
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-restore-secrets
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-restore-secrets
`

// Restore Snapshot Resources
var restoreSnapshotCronJobTemplate = `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: {{ .Name }}-restore-snapshot
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  schedule: 0 */1 * * *
  suspend: true
  jobTemplate:
    metadata:
      labels:
        app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
        app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
    spec:
      template:
        metadata:
          name: consul-restore-snapshot
          labels:
            app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
            app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
        spec:
          restartPolicy: OnFailure
          initContainers:
            - name: consul-acl-bootstrap
              image: docker.io/library/consul:1.7.2
              command:
                - /bin/sh
                - -ec
                - |-
                  secret_dir="/consul/acl"

                  output="$(consul acl bootstrap)"

                  if [ "${output}" = "" ]; then
                    echo "[ERROR] No consul acl bootstrap output. Is consul up and running?"
                    exit 1
                  fi

                  echo "${output}"|grep SecretID|awk '{print $2}'|tr -d '\n' \
                    > "${secret_dir}/secret_id.txt"
              env:
{{- if .Data.TLSGeneratorJobEnabled }}
                - name: CONSUL_HTTP_ADDR
                  value: https://{{ .Name }}-server.{{ .Namespace }}.svc:8500
                - name: CONSUL_CACERT
                  value: /consul/tls/consul-agent-ca.pem
                - name: CONSUL_CLIENT_CERT
                  value: /consul/tls/dc1-cli-consul-0.pem
                - name: CONSUL_CLIENT_KEY
                  value: /consul/tls/dc1-cli-consul-0-key.pem
{{- else }}
                - name: CONSUL_HTTP_ADDR
                  value: http://{{ .Name }}-server.{{ .Namespace }}.svc:8500
{{- end }}
              volumeMounts:
                - name: consul-acl-token
                  mountPath: /consul/acl
{{- if .Data.TLSGeneratorJobEnabled }}
                - name: consul-tls-secret
                  mountPath: /consul/tls
{{- end }}
          containers:
            - name: consul-restore-snapshot
              image: docker.io/library/consul:1.7.2
              command:
                - consul
                - snapshot
                - restore
                - -token-file=/consul/acl/secret_id.txt
                - /consul/restore/backup.snap
              env:
{{- if .Data.TLSGeneratorJobEnabled }}
                - name: CONSUL_HTTP_ADDR
                  value: https://{{ .Name }}-server.{{ .Namespace }}.svc:8500
                - name: CONSUL_CACERT
                  value: /consul/tls/consul-agent-ca.pem
                - name: CONSUL_CLIENT_CERT
                  value: /consul/tls/dc1-cli-consul-0.pem
                - name: CONSUL_CLIENT_KEY
                  value: /consul/tls/dc1-cli-consul-0-key.pem
{{- else }}
                - name: CONSUL_HTTP_ADDR
                  value: http://{{ .Name }}-server.{{ .Namespace }}.svc:8500
{{- end }}
              volumeMounts:
                - name: consul-acl-token
                  mountPath: /consul/acl
{{- if .Data.TLSGeneratorJobEnabled }}
                - name: consul-tls-secret
                  mountPath: /consul/tls
{{- end }}
                - name: restore-secret
                  mountPath: /consul/restore
          volumes:
            - name: consul-acl-token
            - name: restore-secret
              projected:
                sources:
                  - secret:
                      name: {{ .Data.RestoreSecretName }}
{{- if .Data.TLSGeneratorJobEnabled }}
            - name: consul-tls-secret
              projected:
                sources:
                  - secret:
                      name: {{ .Data.TLSCASecretName }}
                  - secret:
                      name: {{ .Data.TLSCLISecretName }}
                  - secret:
                      name: {{ .Data.TLSClientSecretName }}
{{- end }}
`
