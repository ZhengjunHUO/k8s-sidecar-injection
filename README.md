# Kubernetes mutating admission webhook - Inject a sidecar to pod
An instance of kubernetes mutating admission webhook, allowing to inject a sidecar to a pod with specific annotation in labelled namespaces. 
Can be easily expanded to other mutating usages by modifying:
```
func updateReview(review *admv1beta1.AdmissionReview) 
```
in pkg/server/handler.go

## Injection Policy
| # | Title | Tags | 描述 |
| Resource | Label | Enabled value |
| Namespace | istio-injection | enabled |
| Pod | sidecar.istio.io/inject | "true" |

## Environment
- Code written in Go 1.17
- Deployed on kubernetes 1.17.9

## Deployment
```bash
kubectl create ns sidecar-injection
```

### 1) Prepare server image
```bash
docker build -t <REPO/OF/YOUR/IMAGE:TAG> .
docker push <REPO/OF/YOUR/IMAGE:TAG>

# if the private repo is used, a secret should be created in the namespace to allow pulling
kubectl -n sidecar-injection create secret generic regcred --from-file=.dockerconfigjson=/PATH/TO/config.json --type=kubernetes.io/dockerconfigjson
```

### 2a) Issue server's cert via k8s' CA, for apiserver
```bash
# Apply a csr to k8s
make
# Issue the cert with k8s
kubectl certificate approve sidecar-svc-csr
# Save the cert to file
echo $(kubectl get csr sidecar-svc-csr -o jsonpath='{.status.certificate}') | base64 -d > cert/server.crt
# Create cert in target namespace
kubectl create secret generic sidecar-tls --from-file=server.key=cert/server.key --from-file=server.crt=cert/server.crt --dry-run=client -o yaml | kubectl -n sidecar-injection apply -f -
```

### 2b) Inject k8s CA's cabundle to server, for mutual authentication use 
```bash
# Update ${CA_PLACEHOLDER} with the k8s' cabundle
sed -i "s@\${CA_PLACEHOLDER}@$(kubectl get secret -o jsonpath="{.items[?(@.type==\"kubernetes.io/service-account-token\")].data['ca\.crt']}")@" kubernetes/webhook-conf.yaml
# Make sure the ${CA_PLACEHOLDER} is replaced before create a mutatingwebhookconfiguration
kubectl apply -f kubernetes/webhook-conf.yaml
```

### 3) Deploy service 
```bash
# Add sidecar's configuration
kubectl apply -f kubernetes/sidecar-cm.yaml
# Replace container's image: <REPO/OF/YOUR/IMAGE:TAG> before deploy
kubectl apply -f kubernetes/sidecar-deploy.yaml
# Expose the service
kubectl apply -f kubernetes/sidecar-svc.yaml
```

## Test
```bash
# Create namespace for test use
kubectl create ns test-injection
# Label the namespace to match the rule set in webhook configuration
kubectl label namespace test-injection huozj-injection=enabled
# Add config file needed by sidecar
kubectl apply -f kubernetes/nginx-conf.yaml
# Run a pod with annotation to inject a sidecar
kubectl -n test-injection run test --image=alpine --annotations="sidecar.huozj.io/inject=true" --restart=Never -- sleep infinity
```
