{{/*
Expand the name of the chart.
*/}}
{{- define "yandex-messenger-bridge.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "yandex-messenger-bridge.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "yandex-messenger-bridge.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "yandex-messenger-bridge.labels" -}}
helm.sh/chart: {{ include "yandex-messenger-bridge.chart" . }}
{{ include "yandex-messenger-bridge.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "yandex-messenger-bridge.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yandex-messenger-bridge.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "yandex-messenger-bridge.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "yandex-messenger-bridge.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database DSN
*/}}
{{- define "yandex-messenger-bridge.databaseDsn" -}}
{{- printf "postgres://%s:%s@%s:%d/%s?sslmode=%s"
    .Values.database.user
    .Values.database.password
    .Values.database.host
    .Values.database.port
    .Values.database.name
    .Values.database.sslmode
-}}
{{- end }}