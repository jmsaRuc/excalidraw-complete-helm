{{/*
Expand the name of the chart.
*/}}
{{- define "excalidraw-complete.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "excalidraw-complete.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "excalidraw-complete.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}


{{/*
Common labels
*/}}
{{- define "excalidraw-complete.labels" -}}
helm.sh/chart: {{ include "excalidraw-complete.chart" . }}
{{ include "excalidraw-complete.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "excalidraw-complete.selectorLabels" -}}
app.kubernetes.io/name: {{ include "excalidraw-complete.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "excalidraw-complete.postgres.fullname" -}}
{{- if .Values.postgres.fullnameOverride -}}
{{- .Values.postgres.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{ printf "%s-%s" .Release.Name "postgres"}}
{{- end -}}
{{- end -}}

{{/*
Create automatic allowedOrigins value based on `viteFrontendUrl` and `websocketFirebaseHandlerUrl` and `allowedOrigins`.
*/}}
{{- define "excalidraw-complete.allowedOrigins" -}}
{{- if .Values.allowedOrigins }}
{{- printf "%s,%s,%s" .Values.viteFrontendUrl .Values.websocketFirebaseHandlerUrl .Values.allowedOrigins -}}
{{- else -}}
{{- printf "%s,%s" .Values.viteFrontendUrl .Values.websocketFirebaseHandlerUrl -}}
{{- end -}}
{{- end -}}

{{/*
Set postgres host
*/}}
{{- define "excalidraw-complete.postgres.host" -}}
{{- if .Values.postgres.enabled -}}
{{- template "excalidraw-complete.postgres.fullname" . -}}
{{- else -}}
{{- .Values.postgres.host | quote -}}
{{- end -}}
{{- end -}}

{{/*
Set postgres secret
*/}}
{{- define "excalidraw-complete.postgres.secret" -}}
{{- if .Values.postgres.enabled -}}
{{- template "excalidraw-complete.postgres.fullname" . -}}
{{- else -}}
{{- template "excalidraw-complete.fullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Set postgres secretKey
*/}}
{{- define "excalidraw-complete.postgres.secretKey" -}}
{{- if .Values.postgres.enabled -}}
"postgres-password"
{{- else -}}
{{- default "postgres-password" .Values.postgres.auth.secretKeys.passwordKey | quote -}}
{{- end -}}
{{- end -}}


{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "excalidraw-complete.redis.fullname" -}}
{{- if .Values.redis.fullnameOverride -}}
{{- .Values.redis.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{ printf "%s-%s" .Release.Name "redis"}}
{{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "excalidraw-complete.redis.labels" -}}
helm.sh/chart: {{ include "excalidraw-complete.chart" . }}
{{ include "excalidraw-complete.redis.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "excalidraw-complete.redis.selectorLabels" -}}
app.kubernetes.io/name: {{ include "excalidraw-complete.redis.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Set redis host
*/}}
{{- define "excalidraw-complete.redis.host" -}}
{{- if .Values.redis.enabled -}}
{{- template "excalidraw-complete.redis.fullname" . -}}
{{- else -}}
{{- .Values.redis.host | quote -}}
{{- end -}}
{{- end -}}

{{/*
set redis secret
*/}}
{{- define "excalidraw-complete.redis.secret" -}}
{{- if .Values.redis.enabled -}}
{{- template "excalidraw-complete.redis.fullname" . -}}
{{- else -}}
{{- template "excalidraw-complete.fullname" . -}}
{{- end -}}
{{- end -}}

{{/*
set redis secretKey
*/}}
{{- define "excalidraw-complete.redis.secretKey" -}}
{{- if .Values.redis.enabled -}}
"redis-password"
{{- else -}}
{{- default "redis-password" .Values.redis.existingSecretKey | quote -}}
{{- end -}}
{{- end -}}

