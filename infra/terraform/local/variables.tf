variable "cluster_name" {
  description = "kind cluster name (the kubectl context becomes kind-<name>)."
  type        = string
  default     = "bibliomania"
}

variable "node_image" {
  description = "kindest/node image — pins the Kubernetes version."
  type        = string
  default     = "kindest/node:v1.34.0"
}

variable "http_host_port" {
  description = "Host port mapped to the node's :80 (Traefik). Checked in docs/PORTS.md."
  type        = number
  default     = 9097
}

variable "https_host_port" {
  description = "Host port mapped to the node's :443 (Traefik)."
  type        = number
  default     = 9098
}

variable "traefik_chart_version" {
  description = "traefik/traefik Helm chart version (keep in step with infra/k8s/Makefile)."
  type        = string
  default     = "41.6.0"
}

variable "deploy_app" {
  description = "Build/load images and apply infra/k8s/overlays/local after the cluster is up. False = cluster + ingress only."
  type        = bool
  default     = true
}
