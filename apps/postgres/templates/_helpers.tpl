{{- define "postgres.fullname" -}}
{{ .Release.Name }}-postgres
{{- end -}}

{{- define "postgres.labels" -}}
app.kubernetes.io/name: postgres
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
