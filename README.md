# QuObject Controller

A Kubernetes controller specifically designed for integrating QNAP NAS QuObjects S3-compatible storage service with Kubernetes, providing a native Kubernetes interface for applications to request and use object storage buckets from QNAP NAS systems.

## Overview

This controller was built to bring [QNAP QuObjects](https://www.qnap.com/en/how-to/tutorial/article/quobjects-tutorial) - QNAP's S3-compatible object storage service - into the Kubernetes ecosystem. QuObjects allows QNAP NAS users to leverage their existing storage infrastructure as object storage, and this controller makes it seamless to use within Kubernetes clusters.

While designed for QNAP QuObjects, the controller works with any S3-compatible storage (AWS S3, MinIO, Ceph, etc.), making it versatile for various deployment scenarios.

## Features

- **Native QNAP QuObjects Integration**: Purpose-built for seamless integration with QNAP NAS QuObjects service
- **Dynamic Bucket Provisioning**: Automatically creates S3 buckets based on Kubernetes custom resources
- **Flexible Naming**: Support for explicit bucket names or auto-generated names with prefixes
- **Retention Policies**: Choose whether buckets are deleted or retained when claims are removed
- **Credential Management**: Generates and manages Secrets and ConfigMaps with bucket access credentials
- **Multi-Cloud Support**: While designed for QNAP QuObjects, works with any S3-compatible storage (AWS S3, MinIO, Ceph, etc.)
- **SSL/TLS Support**: Configurable HTTPS connections with optional certificate verification
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

The controller needs access to your S3-compatible storage. The secret supports SSL/TLS configuration:

#### For QNAP QuObjects:
```bash
kubectl create secret generic s3-credentials \
  --namespace=quobject-controller \
  --from-literal=endpoint=YOUR-NAS-IP:PORT \
  --from-literal=region=us-east-1 \
  --from-literal=accessKey=YOUR_QUOBJECTS_ACCESS_KEY \
  --from-literal=secretKey=YOUR_QUOBJECTS_SECRET_KEY \
  --from-literal=useSSL=true \
  --from-literal=insecureSkipVerify=true  # Set to true if using self-signed cert

# Example for QNAP with default HTTPS port
kubectl create secret generic s3-credentials \
  --namespace=quobject-controller \
  --from-literal=endpoint=192.168.1.100:9001 \
  --from-literal=region=us-east-1 \
  --from-literal=accessKey=quobjects_user \
  --from-literal=secretKey=your_secret_key \
  --from-literal=useSSL=true \
  --from-literal=insecureSkipVerify=true
```

To obtain QuObjects credentials from your QNAP NAS:
1. Log into your QNAP NAS administration interface
2. Open the QuObjects application
3. Create an access key pair in the QuObjects settings
4. Use the generated access key and secret key in the secret above

For detailed QNAP QuObjects setup, see the [official QNAP QuObjects tutorial](https://www.qnap.com/en/how-to/tutorial/article/quobjects-tutorial).

#### For AWS S3 (HTTPS with certificate verification):
```bash
kubectl create secret generic s3-credentials \
  --namespace=quobject-controller \
  --from-literal=endpoint=s3.amazonaws.com \
  --from-literal=region=us-east-1 \
  --from-literal=accessKey=YOUR_ACCESS_KEY \
  --from-literal=secretKey=YOUR_SECRET_KEY \
  --from-literal=useSSL=true \
  --from-literal=insecureSkipVerify=false
```

#### For MinIO with self-signed certificate:
```bash
kubectl create secret generic s3-credentials \
  --namespace=quobject-controller \
  --from-literal=endpoint=minio.example.com:9000 \
  --from-literal=region=us-east-1 \
  --from-literal=accessKey=minioadmin \
  --from-literal=secretKey=minioadmin \
  --from-literal=useSSL=true \
  --from-literal=insecureSkipVerify=true
```

#### For internal services without SSL:
```bash
kubectl create secret generic s3-credentials \
  --namespace=quobject-controller \
  --from-literal=endpoint=minio.minio.svc.cluster.local:9000 \
  --from-literal=region=us-east-1 \
  --from-literal=accessKey=minioadmin \
  --from-literal=secretKey=minioadmin \
  --from-literal=useSSL=false \
  --from-literal=insecureSkipVerify=false
```

#### SSL Configuration Options:
- **`useSSL`**: `true` (default) uses HTTPS, `false` uses HTTP
- **`insecureSkipVerify`**: `false` (default) verifies certificates, `true` skips verification (for self-signed certs)

### 3. Create a Bucket Claim

#### Example with auto-generated name and delete policy:
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: my-app-bucket
  namespace: default
spec:
  generateBucketName: my-app  # Will create: my-app-xxxxx (random suffix)
  retainPolicy: Delete         # Bucket will be deleted with the claim
  storageClassName: standard
```

#### Example with explicit bucket name and retain policy:
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: production-data
  namespace: default
spec:
  bucketName: prod-data-bucket-2024  # Exact name to use
  retainPolicy: Retain               # Bucket persists after claim deletion (default)
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

## API Reference

### QuObjectBucketClaim

| Field | Type | Description |
|-------|------|-------------|
| `spec.bucketName` | string | Explicit bucket name. If specified, this exact name will be used. |
| `spec.generateBucketName` | string | Prefix for auto-generated bucket names. A 5-character random suffix will be added (e.g., `myapp-x7k2m`) |
| `spec.retainPolicy` | string | `Retain` (default) or `Delete`. Determines if bucket is deleted when claim is removed |
| `spec.storageClassName` | string | Storage class for bucket |
| `spec.additionalConfig` | map[string]string | Additional configuration |
| `status.phase` | string | Current state (Pending/Bound/Error) |
| `status.bucketName` | string | Actual bucket name created |
| `status.secretRef` | string | Name of created Secret |
| `status.configMapRef` | string | Name of created ConfigMap |

### Bucket Naming Behavior

The controller determines bucket names using this precedence:
1. **Explicit name** (`spec.bucketName`): Uses the exact name specified
2. **Generated with prefix** (`spec.generateBucketName`): Adds a 5-character random suffix
3. **Fallback**: If neither is specified, uses `{namespace}-{claim-name}-{random}`

Example outcomes:
- `bucketName: "my-bucket"` → `my-bucket`
- `generateBucketName: "app"` → `app-x7k2m` (random suffix)
- No name specified → `default-my-claim-a9b2c` (namespace-claim-random)

### Retention Policies

| Policy | Behavior |
|--------|----------|
| `Retain` (default) | Bucket persists after claim deletion. Useful for production data. |
| `Delete` | Bucket and all contents are deleted when claim is removed. Useful for temporary/test environments. |

### Generated Secret Fields

| Key | Description |
|-----|-------------|
| `AWS_ACCESS_KEY_ID` | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key |
| `BUCKET_NAME` | Bucket name |
| `BUCKET_HOST` | S3 endpoint |
| `BUCKET_REGION` | S3 region |

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

# Build with specific version tag
IMAGE_TAG=v1.0.0 make ko-build

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

## Configuration

### Controller Configuration

Environment variables for the controller:

| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE` | Controller namespace | quobject-controller |
| `METRICS_ADDR` | Metrics endpoint | :8080 |
| `PROBE_ADDR` | Health probe endpoint | :8081 |
| `LEADER_ELECT` | Enable leader election | false |

### S3 Connection Configuration

The S3 credentials secret (`s3-credentials`) supports:

| Field | Description | Default |
|-------|-------------|---------|
| `endpoint` | S3 endpoint URL | (required) |
| `region` | S3 region | (required) |
| `accessKey` | S3 access key | (required) |
| `secretKey` | S3 secret key | (required) |
| `useSSL` | Use HTTPS (`true`) or HTTP (`false`) | `true` |
| `insecureSkipVerify` | Skip certificate verification | `false` |

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

## Examples

### QNAP QuObjects for Development
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: qnap-dev-bucket
spec:
  generateBucketName: qnap-dev
  retainPolicy: Delete  # Clean up when done
  storageClassName: standard
```

### QNAP QuObjects for Production Storage
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: qnap-prod-data
spec:
  bucketName: company-prod-backup-2024
  retainPolicy: Retain  # Keep bucket for data persistence
  storageClassName: standard
```

### Development Environment with MinIO
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: dev-bucket
spec:
  generateBucketName: dev
  retainPolicy: Delete  # Clean up when done
```

### Production Data Bucket
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: prod-data
spec:
  bucketName: company-prod-data-2024
  retainPolicy: Retain  # Keep bucket even if claim is deleted
```

### Multi-tenant Application
```yaml
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: tenant-bucket
  namespace: tenant-a
spec:
  generateBucketName: tenant-a  # Results in: tenant-a-x7k2m
  retainPolicy: Retain
```

## Troubleshooting

### Common Issues

1. **Controller not creating buckets**
   ```bash
   # Check controller logs
   make logs
   
   # Verify S3 credentials
   kubectl get secret s3-credentials -n quobject-controller -o yaml
   ```

2. **SSL/TLS connection errors**
   ```bash
   # For self-signed certificates, ensure insecureSkipVerify is set
   kubectl edit secret s3-credentials -n quobject-controller
   # Add: insecureSkipVerify: "true" (base64 encoded)
   ```

3. **CRD not recognized**
   ```bash
   # Reinstall CRDs
   make install
   
   # Verify CRDs
   kubectl get crd quobjectbucketclaims.quobject.io
   ```

4. **Bucket not deleted with claim**
   ```bash
   # Check retention policy
   kubectl get quobjectbucketclaim <name> -o jsonpath='{.spec.retainPolicy}'
   # Should be "Delete" for automatic deletion
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

- [x] Support for retention policies
- [x] Auto-generated bucket names with prefixes
- [x] SSL/TLS configuration support
- [ ] Support for bucket policies
- [ ] Bucket size quotas
- [ ] Automatic backup configuration
- [ ] Multi-tenancy improvements
- [ ] Webhook validation
- [ ] Bucket migration support
- [ ] Cost tracking and reporting

## Acknowledgments

This controller was specifically designed to integrate [QNAP QuObjects](https://www.qnap.com/en/how-to/tutorial/article/quobjects-tutorial) S3-compatible storage service with Kubernetes, enabling QNAP NAS users to leverage their existing storage infrastructure in cloud-native applications.

Built with:
- [QNAP QuObjects](https://www.qnap.com/en/how-to/tutorial/article/quobjects-tutorial) - The S3-compatible object storage service that inspired this project
- [Kubebuilder](https://kubebuilder.io) - Kubernetes API framework
- [ko](https://ko.build) - Container image builder for Go
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) - Kubernetes controller library
