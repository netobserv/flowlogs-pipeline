##@ kubernetes

# note: to deploy with custom image tag use: VERSION=test make deploy
.PHONY: deploy
deploy: ## Deploy the image
	sed 's|%IMAGE_TAG_BASE%|$(IMAGE_TAG_BASE)|g;s|%VERSION%|$(VERSION)|g' contrib/kubernetes/deployment.yaml > /tmp/deployment.yaml
	kubectl create configmap flowlogs-pipeline-configuration --from-file=flowlogs-pipeline.conf.yaml=$(FLP_CONF_FILE)
	kubectl apply -f /tmp/deployment.yaml
	kubectl rollout status "deploy/flowlogs-pipeline" --timeout=600s

.PHONY: undeploy
undeploy: ## Undeploy the image
	sed 's|%IMAGE_TAG_BASE%|$(IMAGE_TAG_BASE)|g;s|%VERSION%|$(VERSION)|g' contrib/kubernetes/deployment.yaml > /tmp/deployment.yaml
	kubectl --ignore-not-found=true  delete configmap flowlogs-pipeline-configuration || true
	kubectl --ignore-not-found=true delete -f /tmp/deployment.yaml || true

.PHONY: deploy-loki
deploy-loki: ## Deploy loki
	kubectl apply -f contrib/kubernetes/deployment-loki-storage.yaml
	kubectl apply -f contrib/kubernetes/deployment-loki.yaml
	kubectl rollout status "deploy/loki" --timeout=600s
	-pkill --oldest --full "3100:3100"
	kubectl port-forward --address 0.0.0.0 svc/loki 3100:3100 2>&1 >/dev/null &
	@echo -e "\nloki endpoint is available on http://localhost:3100\n"

.PHONY: undeploy-loki
undeploy-loki: ## Undeploy loki
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-loki.yaml || true
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-loki-storage.yaml || true
	-pkill --oldest --full "3100:3100"

.PHONY: deploy-prometheus
deploy-prometheus: ## Deploy prometheus
	kubectl apply -f contrib/kubernetes/deployment-prometheus.yaml
	kubectl rollout status "deploy/prometheus" --timeout=600s
	-pkill --oldest --full "9090:9090"
	kubectl port-forward --address 0.0.0.0 svc/prometheus 9090:9090 2>&1 >/dev/null &
	@echo -e "\nprometheus ui is available on http://localhost:9090\n"

.PHONY: undeploy-prometheus
undeploy-prometheus: ## Undeploy prometheus
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-prometheus.yaml || true
	-pkill --oldest --full "9090:9090"

.PHONY: deploy-grafana
deploy-grafana: ## Deploy grafana
	kubectl create configmap grafana-dashboard-definitions --from-file=contrib/dashboards/
	kubectl apply -f contrib/kubernetes/deployment-grafana.yaml
	kubectl rollout status "deploy/grafana" --timeout=600s
	-pkill --oldest --full "3000:3000"
	kubectl port-forward --address 0.0.0.0 svc/grafana 3000:3000 2>&1 >/dev/null &
	@echo -e "\ngrafana ui is available (user: admin password: admin) on http://localhost:3000\n"

.PHONY: undeploy-grafana
undeploy-grafana: ## Undeploy grafana
	kubectl --ignore-not-found=true  delete configmap grafana-dashboard-definitions || true
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-grafana.yaml || true
	-pkill --oldest --full "3000:3000"

.PHONY: deploy-netflow-simulator
deploy-netflow-simulator: ## Deploy netflow simulator
	kubectl apply -f contrib/kubernetes/deployment-netflow-simulator.yaml
	kubectl rollout status "deploy/netflowsimulator" --timeout=600s

.PHONY: undeploy-netflow-simulator
undeploy-netflow-simulator: ## Undeploy netflow simulator
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-netflow-simulator.yaml || true

##@ kind

.PHONY: create-kind-cluster
create-kind-cluster: $(KIND) ## Create cluster
	$(KIND) create cluster --name $(KIND_CLUSTER_NAME) --config contrib/kubernetes/kind/kind.config.yaml
	kubectl cluster-info --context kind-kind

.PHONY: delete-kind-cluster
delete-kind-cluster: $(KIND) ## Delete cluster
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-load-image
kind-load-image: ## Load image to kind
ifeq ($(OCI_BIN),$(shell which docker 2>/dev/null))
# This is an optimization for docker provider. "kind load docker-image" can load an image directly from docker's
# local registry. For other providers (i.e. podman), we must use "kind load image-archive" instead.
	$(KIND) load --name $(KIND_CLUSTER_NAME) docker-image $(IMAGE)-${GOARCH}
else
	$(eval tmpfile="/tmp/flp.tar")
	-rm $(tmpfile)
	$(OCI_BIN) save $(IMAGE)-$(GOARCH) -o $(tmpfile)
	$(KIND) load --name $(KIND_CLUSTER_NAME) image-archive $(tmpfile)
	-rm $(tmpfile)
endif

##@ metrics

.PHONY: generate-configuration
generate-configuration: $(KIND) ## Generate metrics configuration
	go build "${CMD_DIR}${CG_BIN_FILE}"
	./${CG_BIN_FILE} --log-level debug --srcFolder network_definitions \
					--destConfFile $(FLP_CONF_FILE) \
					--destDocFile docs/metrics.md \
					--destGrafanaJsonnetFolder contrib/dashboards/jsonnet/

##@ End2End

.PHONY: local-deployments-deploy
local-deployments-deploy: $(KIND) deploy-prometheus deploy-loki deploy-grafana build-image kind-load-image deploy deploy-netflow-simulator
	kubectl get pods
	kubectl rollout status -w deployment/flowlogs-pipeline
	kubectl logs -l app=flowlogs-pipeline

.PHONY: local-deploy
local-deploy: $(KIND) local-cleanup create-kind-cluster local-deployments-deploy ## Deploy locally on kind (with simulated flowlogs)

.PHONY: local-deployments-cleanup
local-deployments-cleanup: $(KIND) undeploy-netflow-simulator undeploy undeploy-grafana undeploy-loki undeploy-prometheus

.PHONY: local-cleanup
local-cleanup: $(KIND) local-deployments-cleanup delete-kind-cluster ## Undeploy from local kind

.PHONY: local-redeploy
local-redeploy:  local-deployments-cleanup local-deployments-deploy ## Redeploy locally (on current kind)

.PHONY: ocp-deploy
ocp-deploy: ocp-cleanup deploy-prometheus deploy-loki deploy-grafana deploy ## Deploy to OCP
	flowlogs_pipeline_svc_ip=$$(kubectl get svc flowlogs-pipeline -o jsonpath='{.spec.clusterIP}'); \
	./hack/enable-ocp-flow-export.sh $$flowlogs_pipeline_svc_ip
	kubectl get pods
	kubectl rollout status -w deployment/flowlogs-pipeline
	kubectl logs -l app=flowlogs-pipeline
	oc expose service prometheus || true
	@prometheus_url=$$(oc get route prometheus -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess prometheus on OCP using: http://"$$prometheus_url"\n"
	oc expose service grafana || true
	@grafana_url=$$(oc get route grafana -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess grafana on OCP using: http://"$$grafana_url"\n"
	oc expose service loki || true
	@loki_url=$$(oc get route loki -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess loki on OCP using: http://"$$loki_url"\n"

.PHONY: ipfix-capture
ipfix-capture: 
	./hack/docker-ipfix.sh

.PHONY: ocp-cleanup
ocp-cleanup: undeploy undeploy-loki undeploy-prometheus undeploy-grafana ## Undeploy from OCP

.PHONY: dev-local-deploy
dev-local-deploy: ## Deploy locally with simulated netflows
	-pkill --oldest --full "${FLP_BIN_FILE}"
	-pkill --oldest --full "${NETFLOW_GENERATOR}"
	go build "${CMD_DIR}${FLP_BIN_FILE}"
	go build "${CMD_DIR}${CG_BIN_FILE}"
	./${CG_BIN_FILE} --log-level debug --srcFolder network_definitions \
	--skipWithTags "kubernetes" \
	--destConfFile /tmp/flowlogs-pipeline.conf.yaml --destGrafanaJsonnetFolder /tmp/
	test -f /tmp/${NETFLOW_GENERATOR} || curl -L --output /tmp/${NETFLOW_GENERATOR}  https://github.com/nerdalert/nflow-generator/blob/master/binaries/nflow-generator-x86_64-linux?raw=true
	chmod +x /tmp/${NETFLOW_GENERATOR}
	./"${FLP_BIN_FILE}" --config /tmp/flowlogs-pipeline.conf.yaml &
	/tmp/${NETFLOW_GENERATOR} -t 127.0.0.1 -p 2056 > /dev/null 2>&1 &