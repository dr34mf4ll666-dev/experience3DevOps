{{/* 通用 helpers */}}

{{- define "nekocafe.fullname" -}}
{{- printf "%s-%s" .Release.Name .name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nekocafe.labels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: nekocafe
app.kubernetes.io/component: {{ .name }}
nekocafe.io/environment: {{ .Values.global.environment }}
{{- end -}}

{{- define "nekocafe.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "nekocafe.image" -}}
{{ .Values.global.imageRegistry }}/{{ .image.repository }}:{{ .image.tag | default .Chart.AppVersion }}
{{- end -}}
