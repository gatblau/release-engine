# Crossplane bootstrap for `InfrastructureRequest`

This folder contains a minimal Crossplane control-plane configuration so the
generated resource below can be accepted by the API server:

```yaml
apiVersion: infrastructure.platform.io/v1alpha1
kind: InfrastructureRequest
```

## Files

- `infrastructurerequest-xrd.yaml`: defines the XRD/CRD surface.
- `composition-k8s-application-v1.yaml`: composition for template `k8s-app`.
- `composition-data-proc-v1.yaml`: composition for template `data-proc`.
- `composition-vm-app-v1.yaml`: composition for template `vm-app`.
- `function-patch-and-transform.yaml`: required composition function package.

## Apply order

```bash
kubectl apply -f docs/modules/infra/crossplane/function-patch-and-transform.yaml
kubectl apply -f docs/modules/infra/crossplane/infrastructurerequest-xrd.yaml
kubectl apply -f docs/modules/infra/crossplane/composition-k8s-application-v1.yaml
kubectl apply -f docs/modules/infra/crossplane/composition-data-proc-v1.yaml
kubectl apply -f docs/modules/infra/crossplane/composition-vm-app-v1.yaml
```

Then apply your generated resource, for example:

```bash
kubectl apply -f internal/integration/testdata/output/infra-k8s-app.manifest.yaml
kubectl apply -f internal/integration/testdata/output/infra-data-proc.manifest.yaml
kubectl apply -f internal/integration/testdata/output/infra-vm-app.manifest.yaml
```

## Notes

- This is a **minimal skeleton** to make the custom resource resolvable and composable.
- It does **not** yet provision cloud managed resources.
- Next step is adding concrete composed resources (EKS, RDS, etc.) with provider configs.