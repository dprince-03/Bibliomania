# Bibliomania on a local kind cluster. Run from anywhere:
#   make -C infra/k8s up        # cluster + Traefik + images + secrets + apply + wait
#   make -C infra/k8s seed      # admin + sample catalog (port-forwards for you)
#   make -C infra/k8s smoke     # the same 41-check end-to-end test as Compose
#   make -C infra/k8s down      # delete the cluster
# Terraform (infra/terraform/local) does `up` declaratively instead.
#
# Needs: docker, kind, kubectl, helm (and openssl for `secrets`).

CLUSTER   ?= bibliomania
OVERLAY   ?= local
NAMESPACE := bibliomania
REPO_ROOT := $(abspath $(CURDIR)/../..)
APP_DIR   := $(REPO_ROOT)/Server/app
KUSTOMIZE := kubectl kustomize --load-restrictor LoadRestrictionsNone
KCTL      := kubectl --context kind-$(CLUSTER) -n $(NAMESPACE)

GO_SERVICES := gateway auth catalog borrow reading user notification payment
TRAEFIK_CHART_VERSION := 41.6.0

# Third-party images the manifests use. `preload` copies any the host
# already has into the kind node (skipping missing ones), so a fresh cluster
# doesn't re-pull them all from registries.
THIRD_PARTY_IMAGES := mysql:8.4 postgres:16-alpine mongo:7 redis:7-alpine \
	rabbitmq:4.2-management-alpine apache/kafka:3.9.1 nats:2.11-alpine \
	prom/prometheus:v3.5.0 prom/alertmanager:v0.28.1 grafana/loki:3.5.3 grafana/tempo:2.8.2 grafana/alloy:v1.10.2 grafana/grafana:12.1.1 \
	prom/mysqld-exporter:v0.17.2 quay.io/prometheuscommunity/postgres-exporter:v0.17.1 \
	percona/mongodb_exporter:0.44 oliver006/redis_exporter:v1.74.0 danielqsj/kafka-exporter:v1.9.0 \
	natsio/prometheus-nats-exporter:0.17.3 ghcr.io/umami-software/umami:postgresql-latest axllent/mailpit:latest \
	chrislusf/seaweedfs:4.47 clamav/clamav:1.4 rclone/rclone:1.71 ghcr.io/cloudnative-pg/postgresql:16

CNPG_CHART_VERSION := 0.29.1
LINKERD_CHART_VERSION := 2026.9.3
SEALED_SECRETS_CHART_VERSION := 2.20.0
ARGOCD_CHART_VERSION := 10.9.2
ARGOCD_CONTEXT ?= kind-$(CLUSTER)
# seal: which cluster's controller will decrypt (set to the production
# context for real use), and where the plaintext / sealed files live.
SEAL_CONTEXT ?= kind-$(CLUSTER)
SEAL_SRC ?= overlays/prod/secrets
SEAL_OUT ?= overlays/prod/sealed
MESH_CERTS := mesh-certs
CERT_MANAGER_VERSION := v1.21.2
RABBITMQ_OPERATOR_VERSION := v2.23.0

.PHONY: up up-ha operators mesh mesh-certs sealed-secrets seal argocd down cluster ingress images images-go load load-go preload secrets render apply wait status seed smoke

up: cluster ingress images load preload secrets apply wait
	@echo "Up. Gateway via Traefik: curl -H 'Host: api.bibliomania.local' http://localhost:9097/health"

# HA variant: operators + overlays/local-ha (see components/ha).
up-ha:
	$(MAKE) cluster ingress operators mesh images load preload secrets
	$(MAKE) apply wait OVERLAY=local-ha

# Operators components/ha relies on: cert-manager (the RabbitMQ operator's
# webhook certificates — and TLS for overlays/prod), CloudNativePG
# (Postgres) and the RabbitMQ Cluster Operator.
operators:
	helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
	helm repo update jetstack >/dev/null
	helm upgrade --install cert-manager jetstack/cert-manager --version $(CERT_MANAGER_VERSION) \
		--kube-context kind-$(CLUSTER) --namespace cert-manager --create-namespace \
		--set crds.enabled=true --wait
	helm repo add cnpg https://cloudnative-pg.github.io/charts >/dev/null 2>&1 || true
	helm repo update cnpg >/dev/null
	helm upgrade --install cnpg cnpg/cloudnative-pg --version $(CNPG_CHART_VERSION) \
		--kube-context kind-$(CLUSTER) --namespace cnpg-system --create-namespace --wait
	kubectl --context kind-$(CLUSTER) apply -f \
		https://github.com/rabbitmq/cluster-operator/releases/download/$(RABBITMQ_OPERATOR_VERSION)/cluster-operator.yml
	kubectl --context kind-$(CLUSTER) -n rabbitmq-system rollout status deployment/rabbitmq-cluster-operator --timeout=180s

cluster:
	@kind get clusters | grep -qx $(CLUSTER) || kind create cluster --name $(CLUSTER) --config kind/cluster.yaml

ingress:
	helm repo add traefik https://traefik.github.io/charts >/dev/null 2>&1 || true
	helm repo update traefik >/dev/null
	helm upgrade --install traefik traefik/traefik --version $(TRAEFIK_CHART_VERSION) \
		--kube-context kind-$(CLUSTER) --namespace traefik --create-namespace \
		-f kind/traefik-values.yaml --wait

# Prod Dockerfiles, tagged :dev — the same images CD pushes, built locally.
# images-go / load-go: just the eight Go services — the usual loop after a
# backend change, without rebuilding the four client apps.
images: images-go
	docker build -q -f $(REPO_ROOT)/infra/docker/web-app/Dockerfile.prod -t bibliomania-web-app:dev $(REPO_ROOT)/Client/web/app
	docker build -q -f $(REPO_ROOT)/infra/docker/web-main/Dockerfile.prod -t bibliomania-web-main:dev \
		--build-arg NEXT_PUBLIC_API_URL=http://api.bibliomania.local:9097 \
		--build-arg NEXT_PUBLIC_APP_URL=http://app.bibliomania.local:9097 $(REPO_ROOT)/Client/web/main
	docker build -q -f $(REPO_ROOT)/infra/docker/admin-web/Dockerfile.prod -t bibliomania-admin-web:dev $(REPO_ROOT)/Client/admin
	docker build -q -f $(REPO_ROOT)/infra/docker/admin/Dockerfile.prod -t bibliomania-admin:dev $(REPO_ROOT)/Server/admin

images-go:
	@for s in $(GO_SERVICES); do \
		echo "== bibliomania-$$s"; \
		docker build -q -f $(REPO_ROOT)/infra/docker/app/Dockerfile.prod --build-arg SERVICE=$$s \
			-t bibliomania-$$s:dev $(APP_DIR) || exit 1; \
	done

load-go:
	kind load docker-image --name $(CLUSTER) $(foreach s,$(GO_SERVICES),bibliomania-$(s):dev)

load:
	kind load docker-image --name $(CLUSTER) \
		$(foreach s,$(GO_SERVICES),bibliomania-$(s):dev) \
		bibliomania-web-app:dev bibliomania-web-main:dev bibliomania-admin-web:dev bibliomania-admin:dev

# Linkerd (components/mesh): mTLS between services + gRPC load balancing.
# mesh-certs makes the mesh's own CA (trust anchor, 10 years) and the
# intermediate the identity service issues pod certificates from (1 year) —
# ECDSA P-256, as Linkerd requires. Written once to $(MESH_CERTS)/
# (git-ignored); rotate the issuer yearly. Production: have cert-manager
# issue and rotate the issuer instead (see infra/k8s/README.md).
mesh-certs:
	@mkdir -p $(MESH_CERTS); cd $(MESH_CERTS); \
	if [ ! -f ca.crt ]; then \
		openssl ecparam -name prime256v1 -genkey -noout -out ca.key && \
		openssl req -x509 -new -key ca.key -days 3650 -subj "/CN=root.linkerd.cluster.local" \
			-addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" -out ca.crt && \
		openssl ecparam -name prime256v1 -genkey -noout -out issuer.key && \
		openssl req -new -key issuer.key -subj "/CN=identity.linkerd.cluster.local" -out issuer.csr && \
		printf "basicConstraints=critical,CA:TRUE,pathlen:0\nkeyUsage=critical,keyCertSign,cRLSign\n" > issuer.ext && \
		openssl x509 -req -in issuer.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -extfile issuer.ext -out issuer.crt && \
		rm -f issuer.csr issuer.ext ca.srl && echo "mesh certificates written to $(MESH_CERTS)/"; \
	fi

mesh: mesh-certs
	helm repo add linkerd-edge https://helm.linkerd.io/edge >/dev/null 2>&1 || true
	helm repo update linkerd-edge >/dev/null
	helm upgrade --install linkerd-crds linkerd-edge/linkerd-crds --version $(LINKERD_CHART_VERSION) \
		--kube-context kind-$(CLUSTER) -n linkerd --create-namespace --wait
	helm upgrade --install linkerd-control-plane linkerd-edge/linkerd-control-plane --version $(LINKERD_CHART_VERSION) \
		--kube-context kind-$(CLUSTER) -n linkerd --wait --timeout 10m \
		--set-file identityTrustAnchorsPEM=$(MESH_CERTS)/ca.crt \
		--set-file identity.issuer.tls.crtPEM=$(MESH_CERTS)/issuer.crt \
		--set-file identity.issuer.tls.keyPEM=$(MESH_CERTS)/issuer.key

# Sealed Secrets: production Secrets live in git encrypted (SealedSecret),
# decryptable only by the controller in the target cluster — so Argo CD can
# deploy everything from the repo with no secret ever committed in plaintext.
sealed-secrets:
	helm repo add sealed-secrets https://bitnami.github.io/sealed-secrets >/dev/null 2>&1 || true
	helm repo update sealed-secrets >/dev/null
	helm upgrade --install sealed-secrets sealed-secrets/sealed-secrets --version $(SEALED_SECRETS_CHART_VERSION) \
		--kube-context $(SEAL_CONTEXT) -n kube-system --set fullnameOverride=sealed-secrets-controller --wait

# $(SEAL_SRC)/<name>.env (git-ignored plaintext) → $(SEAL_OUT)/<name>.yaml.
# pg-* files become kubernetes.io/basic-auth (what CloudNativePG expects).
seal:
	@ls $(SEAL_SRC)/*.env >/dev/null 2>&1 || { echo "no $(SEAL_SRC)/*.env — see overlays/prod/secrets/README.md"; exit 1; }
	@cert=$$(mktemp); trap 'rm -f $$cert' EXIT; \
	kubeseal --context $(SEAL_CONTEXT) --fetch-cert > $$cert || exit 1; \
	mkdir -p $(SEAL_OUT); rm -f $(SEAL_OUT)/*.yaml.new; \
	for f in $(SEAL_SRC)/*.env; do \
		n=$$(basename $$f .env); type=Opaque; case $$n in pg-*) type=kubernetes.io/basic-auth;; esac; \
		kubectl create secret generic $$n -n bibliomania --type=$$type --from-env-file=$$f --dry-run=client -o yaml \
			| kubeseal --cert $$cert --format yaml > $(SEAL_OUT)/$$n.yaml || exit 1; \
		echo "sealed $$n"; \
	done; \
	{ sed -n '1,/^kind: Kustomization/p' $(SEAL_OUT)/kustomization.yaml; echo "resources:"; \
	  for f in $(SEAL_OUT)/*.yaml; do b=$$(basename $$f); [ $$b = kustomization.yaml ] || echo "  - $$b"; done; } > $(SEAL_OUT)/kustomization.yaml.new; \
	mv $(SEAL_OUT)/kustomization.yaml.new $(SEAL_OUT)/kustomization.yaml

# Argo CD + the production Application (argocd/). Point ARGOCD_CONTEXT at
# the production cluster; admin password:
#   kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
argocd:
	helm repo add argo https://argoproj.github.io/argo-helm >/dev/null 2>&1 || true
	helm repo update argo >/dev/null
	helm upgrade --install argocd argo/argo-cd --version $(ARGOCD_CHART_VERSION) \
		--kube-context $(ARGOCD_CONTEXT) -n argocd --create-namespace -f argocd/values.yaml --wait --timeout 10m
	kubectl --context $(ARGOCD_CONTEXT) apply -f argocd/application-prod.yaml

# `kind load` imports with --all-platforms, which fails for multi-arch
# images when Docker's containerd image store only holds this machine's
# platform ("content digest ... not found"). Fall back to importing just
# linux/amd64 straight into the node's containerd.
preload:
	@for img in $(THIRD_PARTY_IMAGES); do \
		docker image inspect $$img >/dev/null 2>&1 || continue; \
		if kind load docker-image --name $(CLUSTER) $$img >/dev/null 2>&1 || \
		   docker save $$img | docker exec -i $(CLUSTER)-control-plane ctr --namespace=k8s.io images import --platform linux/amd64 --digests --snapshotter=overlayfs - >/dev/null 2>&1; \
		then echo "preloaded $$img"; else echo "could not preload $$img (the node will pull it)"; fi; \
	done

# overlays/local/secrets.env (git-ignored) from the example, with a fresh
# random JWT secret. An existing file is never overwritten — but keys added
# to the example since it was created are appended (with the example's
# value), so new services don't fail on a missing Secret key.
secrets:
	@if [ ! -f overlays/local/secrets.env ]; then \
		sed -e "s/^JWT_SECRET=.*/JWT_SECRET=$$(openssl rand -hex 32)/" -e "s/^INTERNAL_API_TOKEN=.*/INTERNAL_API_TOKEN=$$(openssl rand -hex 32)/" overlays/local/secrets.env.example > overlays/local/secrets.env; \
		echo "created overlays/local/secrets.env"; \
	else \
		grep -E '^[A-Z0-9_]+=' overlays/local/secrets.env.example | while IFS= read -r line; do \
			key=$${line%%=*}; \
			grep -q "^$$key=" overlays/local/secrets.env || { echo "$$line" >> overlays/local/secrets.env; echo "added $$key to overlays/local/secrets.env"; }; \
		done; \
	fi
	@# HA overlay: CloudNativePG basic-auth Secrets, same passwords.
	@mkdir -p overlays/local-ha/ha-secrets; \
	pw() { grep "^$$1=" overlays/local/secrets.env | cut -d= -f2-; }; \
	printf 'username=postgres\npassword=%s\n' "$$(pw POSTGRES_PASSWORD)" > overlays/local-ha/ha-secrets/pg-superuser.env; \
	printf 'username=catalog\npassword=%s\n' "$$(pw CATALOG_DB_PASSWORD)" > overlays/local-ha/ha-secrets/pg-catalog.env; \
	printf 'username=borrow\npassword=%s\n' "$$(pw BORROW_DB_PASSWORD)" > overlays/local-ha/ha-secrets/pg-borrow.env; \
	printf 'username=users\npassword=%s\n' "$$(pw USER_DB_PASSWORD)" > overlays/local-ha/ha-secrets/pg-users.env; \
	printf 'username=payment\npassword=%s\n' "$$(pw PAYMENT_DB_PASSWORD)" > overlays/local-ha/ha-secrets/pg-payment.env

render:
	@$(KUSTOMIZE) overlays/$(OVERLAY)

apply:
	$(KUSTOMIZE) overlays/$(OVERLAY) | kubectl --context kind-$(CLUSTER) apply -f -

wait:
	$(KCTL) rollout status statefulset --timeout=300s
	$(KCTL) wait --for=condition=complete job/mongo-rs-init --timeout=300s
	$(KCTL) rollout status deployment --timeout=600s

status:
	$(KCTL) get pods,svc,ingress

# seed and smoke reach the cluster through temporary port-forwards on
# 19081 (gateway), 19086 (auth MySQL), 19089 (Mailpit UI).
seed:
	@$(KCTL) port-forward svc/gateway 19081:8080 >/dev/null & pf1=$$!; \
	 $(KCTL) port-forward pod/mysql-0 19086:3306 >/dev/null & pf2=$$!; \
	 sleep 3; \
	 AUTH_PW=$$(grep ^AUTH_DB_PASSWORD= overlays/local/secrets.env | cut -d= -f2-); \
	 cd $(APP_DIR) && SEED_AUTH_DATABASE_URL="auth:$$AUTH_PW@tcp(localhost:19086)/bibliomania_auth" \
	   SEED_GATEWAY_URL=http://localhost:19081 go run ./cmd/seed; rc=$$?; \
	 kill $$pf1 $$pf2; exit $$rc

smoke:
	@$(KCTL) port-forward svc/gateway 19081:8080 >/dev/null & pf1=$$!; \
	 $(KCTL) port-forward svc/mailpit 19089:8025 >/dev/null & pf2=$$!; \
	 sleep 3; \
	 $(REPO_ROOT)/infra/docker/smoke-test.sh http://localhost:19081 http://localhost:19089; rc=$$?; \
	 kill $$pf1 $$pf2; exit $$rc

down:
	kind delete cluster --name $(CLUSTER)
