output "kubeconfig_path" {
  description = "Kubeconfig for the cluster (kubectl --kubeconfig <path>)."
  value       = kind_cluster.this.kubeconfig_path
}

output "kube_context" {
  value = "kind-${var.cluster_name}"
}

output "gateway_url" {
  description = "The API through Traefik (send Host: api.bibliomania.local, or add it to /etc/hosts)."
  value       = "http://api.bibliomania.local:${var.http_host_port}"
}
