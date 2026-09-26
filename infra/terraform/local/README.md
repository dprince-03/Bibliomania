# Terraform: local Kubernetes

Creates the kind cluster, installs Traefik (Helm) and deploys
`infra/k8s/overlays/local`. The declarative twin of `make -C infra/k8s up`,
using the same cluster settings, the same Traefik values file and the same
Make targets for images and manifests.

```bash
terraform init
terraform apply                      # ~10 min first time (image builds + pulls)
terraform apply -var deploy_app=false  # cluster + ingress only
terraform destroy
```

Then `make -C infra/k8s seed` and `make -C infra/k8s smoke`. Details and
the reasons behind the tool choices: [`infra/k8s/README.md`](../../k8s/README.md).

- State is local (`terraform.tfstate`, git-ignored — it holds the
  cluster's credentials). `.terraform.lock.hcl` is committed.
- The app step re-runs whenever a file under `infra/k8s/base` or
  `infra/k8s/overlays/local` changes. After changing only Go code, rebuild
  the images with `terraform apply -replace='null_resource.app[0]'` or
  `make -C infra/k8s images load` plus a `rollout restart`.
- Moving to a cloud provider: replace `kind_cluster` with that provider's
  managed-cluster resource (e.g. DigitalOcean `digitalocean_kubernetes_cluster`),
  point the Helm provider at it, and apply `overlays/prod` instead.
