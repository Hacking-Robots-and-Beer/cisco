{{/*
Common name
*/}}
{{- define "cisco.name" -}}
cisco-aps
{{- end }}

{{/*
Common labels
*/}}
{{- define "cisco.labels" -}}
app.kubernetes.io/managed-by: Helm
app.kubernetes.io/part-of: cisco-aps
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cisco.selectorLabels" -}}
app.kubernetes.io/name: {{ . }}
app.kubernetes.io/part-of: cisco-aps
{{- end }}
