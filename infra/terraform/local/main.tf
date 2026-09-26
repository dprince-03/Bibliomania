# Local Kubernetes for Bibliomania, declaratively: a kind cluster, Traefik
# as the ingress controller, then the app (infra/k8s/overlays/local).
#
#   cd infra/terraform/local && terraform init && terraform apply
#
# Mirrors `make -C infra/k8s up` (same cluster settings as
# infra/k8s/kind/cluster.yaml, same Traefik values file, same Make targets
# for images + manifests) — Terraform owns the lifecycle, Kustomize owns
# the manifests. The planned move to a real provider (DigitalOcean/Hetzner
# managed Kubernetes, see Server/app/docs/plan.md) swaps kind_cluster for
# that provider's cluster resource; the helm_release and overlay apply
# stay the same.

locals {
  k8s_dir = abspath("${path.module}/../../k8s")
  manifest_files = sort(concat(
    [for f in fileset(local.k8s_dir, "base/**") : "${local.k8s_dir}/${f}"],
    [for f in fileset(local.k8s_dir, "overlays/local/**") : "${local.k8s_dir}/${f}"],
  ))
}

resource "kind_cluster" "this" {
  name           = var.cluster_name
  node_image     = var.node_image
  wait_for_ready = true

  kind_config {
    kind        = "Cluster"
    api_version = "kind.x-k8s.io/v1alpha4"

    node {
      role = "control-plane"

      kubeadm_config_patches = [
        <<-EOT
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
        EOT
      ]

      extra_port_mappings {
        container_port = 80
        host_port      = var.http_host_port
        protocol       = "TCP"
      }
      extra_port_mappings {
        container_port = 443
        host_port      = var.https_host_port
        protocol       = "TCP"
      }
    }
  }
}

provider "helm" {
  kubernetes = {
    host                   = kind_cluster.this.endpoint
    client_certificate     = kind_cluster.this.client_certificate
    client_key             = kind_cluster.this.client_key
    cluster_ca_certificate = kind_cluster.this.cluster_ca_certificate
  }
}

resource "helm_release" "traefik" {
  name             = "traefik"
  repository       = "https://traefik.github.io/charts"
  chart            = "traefik"
  version          = var.traefik_chart_version
  namespace        = "traefik"
  create_namespace = true
  wait             = true
  values           = [file("${local.k8s_dir}/kind/traefik-values.yaml")]
}

# Images + manifests via the same Make targets `make -C infra/k8s up`
# uses. Re-runs whenever any manifest or the overlay changes.
resource "null_resource" "app" {
  count      = var.deploy_app ? 1 : 0
  depends_on = [helm_release.traefik]

  triggers = {
    cluster   = kind_cluster.this.id
    manifests = sha1(join("", [for f in local.manifest_files : filesha1(f)]))
  }

  provisioner "local-exec" {
    command = "make -C ${local.k8s_dir} images load preload secrets apply wait CLUSTER=${var.cluster_name}"
    environment = {
      KUBECONFIG = kind_cluster.this.kubeconfig_path
    }
  }
}
