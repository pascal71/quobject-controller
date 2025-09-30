# QuObject Controller

A Kubernetes controller for managing S3-compatible object storage bucket claims from QNAP using QuObject service, providing a simplified interface for applications to request and use object storage buckets.

## Features

- **Dynamic Bucket Provisioning**: Automatically creates S3 buckets based on Kubernetes custom resources
- **Credential Management**: Generates and manages Secrets and ConfigMaps with bucket access credentials
- **Multi-Cloud Support**: Works with any S3-compatible storage (AWS S3, MinIO, Ceph, etc.)
- **Multi-Architecture**: Supports both amd64 and arm64 architectures
- **Lifecycle Management**: Handles bucket cleanup through Kubernetes finalizers

## Architecture

The QuObject Controller watches for `QuObjectBucketClaim` custom resources and:
1. Creates S3 buckets in your configured object storage
2. Generates Kubernetes Secrets with access credentials
3. Creates ConfigMaps with bucket configuration
4. Manages the lifecycle of these resources

## Prerequisites

- Kubernetes cluster (v1.24+)
- Go 1.24+ (for development)
- S3-compatible object storage with credentials
- kubectl configured to access your cluster
- (Optional) ko for container builds

## Quick Start

### 1. Install the Controller

```bash
# Clone the repository
git clone https://github.com/pamvdam71/quobject-controller
cd quobject-controller

# Install CRDs and deploy the controller
make quickstart
```

### 2. Configure S3 Credentials

The controller needs access to your S3-compatible storage:

```bash
# Interactive setup
make create-s3-secret

# Or manually
kubectl create secret generic s3-credentials \
  --namespace=quobject-controller \
  --from-literal=endpoint=s3.amazonaws.com \
  --from-literal=region=us-east-1 \
  --from-literal=accessKey=YOUR_ACCESS_KEY \
  --from-literal=secretKey=YOUR_SECRET_KEY
```

### 3. Create a Bucket Claim

```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: my-app-bucket
  namespace: default
spec:
  generateBucketName: my-app
  storageClassName: standard
```

Apply it:
```bash
kubectl apply -f examples/bucket-claim.yaml
```

### 4. Use the Generated Resources

The controller creates:
- A Secret named `{claim-name}-bucket-secret` with credentials
- A ConfigMap named `{claim-name}-bucket-config` with bucket details

```yaml
# In your application pod
apiVersion: v1
kind: Pod
metadata:
  name: my-app
spec:
  containers:
  - name: app
    image: my-app:latest
    envFrom:
    - secretRef:
        name: my-app-bucket-bucket-secret
    - configMapRef:
        name: my-app-bucket-bucket-config
```

## Development

### Building from Source

```bash
# Build binary
make build

# Run tests
make test

# Run locally (outside cluster)
make run
```

### Building Container Images

The project uses [ko](https://ko.build) for building minimal, multi-arch container images:

```bash
# Set your registry
export KO_DOCKER_REPO=quay.io/yourusername/quobject-controller

# Build and push multi-arch image
make ko-build

# Generate Kubernetes manifests
make ko-resolve
```

### Traditional Docker Build

```bash
# Single architecture
make docker-build docker-push

# Multi-architecture with buildx
make docker-buildx
```

## API Reference

### QuObjectBucketClaim

| Field | Type | Description |
|-------|------|-------------|
| `spec.bucketName` | string | Explicit bucket name (optional) |
| `spec.generateBucketName` | string | Prefix for generated bucket name |
| `spec.storageClassName` | string | Storage class for bucket |
| `spec.additionalConfig` | map[string]string | Additional configuration |
| `status.phase` | string | Current state (Pending/Bound/Error) |
| `status.bucketName` | string | Actual bucket name created |
| `status.secretRef` | string | Name of created Secret |
| `status.configMapRef` | string | Name of created ConfigMap |

### Generated Secret Fields

| Key | Description |
|-----|-------------|
| `AWS_ACCESS_KEY_ID` | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key |
| `BUCKET_NAME` | Bucket name |
| `BUCKET_HOST` | S3 endpoint |
| `BUCKET_REGION` | S3 region |

## Configuration

### Controller Configuration

Environment variables for the controller:

| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE` | Controller namespace | quobject-controller |
| `METRICS_ADDR` | Metrics endpoint | :8080 |
| `PROBE_ADDR` | Health probe endpoint | :8081 |
| `LEADER_ELECT` | Enable leader election | false |

### Makefile Configuration

Key variables in the Makefile:

```bash
# Registry configuration
KO_DOCKER_REPO=quay.io/yourusername/quobject-controller
IMAGE_TAG=latest

# Target platforms
KO_PLATFORMS=linux/amd64,linux/arm64

# Kubernetes namespace
NAMESPACE=quobject-controller
```

## Monitoring

### Logs

```bash
# View controller logs
make logs

# Follow logs
kubectl logs -n quobject-controller -l control-plane=controller-manager -f
```

### Metrics

The controller exposes Prometheus metrics on port 8080:
- Reconciliation duration
- Reconciliation errors
- Bucket creation success/failure

### Health Checks

- Liveness: `:8081/healthz`
- Readiness: `:8081/readyz`

## Troubleshooting

### Common Issues

1. **Controller not creating buckets**
   ```bash
   # Check controller logs
   make logs
   
   # Verify S3 credentials
   kubectl get secret s3-credentials -n quobject-controller -o yaml
   ```

2. **CRD not recognized**
   ```bash
   # Reinstall CRDs
   make install
   
   # Verify CRDs
   kubectl get crd quobjectbucketclaims.quobject.io
   ```

3. **Image pull errors**
   ```bash
   # Verify image exists
   make verify-image
   
   # Check registry credentials
   kubectl get secret -n quobject-controller | grep docker
   ```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Update documentation
6. Submit a pull request

### Code Generation

This project uses code generation for:
- DeepCopy methods: `make generate`
- CRD manifests: `make manifests`

Always run these before committing API changes.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/pamvdam71/quobject-controller/issues)
- **Documentation**: [Wiki](https://github.com/pamvdam71/quobject-controller/wiki)

## Roadmap

- [ ] Support for bucket policies
- [ ] Bucket size quotas
- [ ] Automatic backup configuration
- [ ] Multi-tenancy improvements
- [ ] Webhook validation
- [ ] Bucket migration support
- [ ] Cost tracking and reporting

## Acknowledgments

Built with:
- [Kubebuilder](https://kubebuilder.io) - Kubernetes API framework
- [ko](https://ko.build) - Container image builder for Go
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) - Kubernetes controller library
