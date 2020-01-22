package consul

var sidecarPatchTemplate = `apiVersion: {{ .APIVersion }}
kind: {{ .Kind }}
metadata:
  name: {{ .ObjectMeta.Name }}
  namespace: {{ .ObjectMeta.Namespace }}
spec:
  template:
    spec:
      containers:
        - name: consul-agent
          image: docker.io/library/consul:1.6.2
          command:
            - consul
            - agent
            - -bind=$(POD_IP)
            - -retry-join={{ .ConsulServiceFQDN }}
            - -data-dir=/consul/data
            - -config-dir=/consul/configs
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - name: consul-data
              mountPath: /consul/data
            - name: consul-configs
              mountPath: /consul/configs
            - name: consul-tls-secret
              mountPath: /consul/tls
      volumes:
        - name: consul-data
          emptyDir: {}
        - name: consul-tls
          emptyDir: {}
        - name: consul-configs
          projected:
            sources:
              - configMap:
                  name: {{ .ConsulName }}-{{ .ConsulNamespace }}-tls
                  optional: true
        - name: consul-tls-secret
          secret:
            secretName: {{ .ConsulName }}-{{ .ConsulNamespace }}-tls-ca
            optional: true
`

var sidecarTLSCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .ConsulName }}-{{ .ConsulNamespace }}-tls
  namespace: {{ .ObjectMeta.Namespace }}
data:
  00-default-agent-tls.json: |-
    {
      "auto_encrypt": {
        "tls": true
      },
      "ca_file": "/consul/tls/consul-agent-ca.pem",
      "ports": {
        "http": -1,
        "https": 8501
      }
    }
`
