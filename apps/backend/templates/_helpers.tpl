{{- define "backend.fullname" -}}
{{ .Release.Name }}-backend
{{- end -}}

{{- define "backend.labels" -}}
app.kubernetes.io/name: backend
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
