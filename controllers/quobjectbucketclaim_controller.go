package controllers

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quv1 "github.com/pamvdam71/quobject-controller/api/v1alpha1"
)

const (
	finalizerName = "quobject.io/finalizer"
	controllerNS  = "quobject-controller"
	
	// Annotations for storing bucket metadata
	annotationBucketName = "quobject.io/bucket-name"
	annotationRetainPolicy = "quobject.io/retain-policy"
)

// QuObjectBucketClaimReconciler reconciles a QuObjectBucketClaim object
type QuObjectBucketClaimReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=quobject.io,resources=quobjectbucketclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=quobject.io,resources=quobjectbucketclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=quobject.io,resources=quobjectbucketclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for QuObjectBucketClaim resources
func (r *QuObjectBucketClaimReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the QuObjectBucketClaim instance
	claim := &quv1.QuObjectBucketClaim{}
	err := r.Get(ctx, req.NamespacedName, claim)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("QuObjectBucketClaim not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get QuObjectBucketClaim")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !claim.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, claim)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(claim, finalizerName) {
		controllerutil.AddFinalizer(claim, finalizerName)
		if err := r.Update(ctx, claim); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Main reconciliation logic
	log.Info("Reconciling QuObjectBucketClaim", "Name", claim.Name, "Namespace", claim.Namespace)

	// Get S3 credentials from secret
	credSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      "s3-credentials",
		Namespace: controllerNS,
	}, credSecret)
	if err != nil {
		log.Error(err, "Failed to get S3 credentials secret")
		claim.Status.Phase = "Error"
		r.Status().Update(ctx, claim)
		return ctrl.Result{}, err
	}

	// Extract credentials
	endpoint := string(credSecret.Data["endpoint"])
	region := string(credSecret.Data["region"])
	accessKey := string(credSecret.Data["accessKey"])
	secretKey := string(credSecret.Data["secretKey"])

	// Create S3 client
	s3Client, err := newS3Client(endpoint, region, accessKey, secretKey, true, true)
	if err != nil {
		log.Error(err, "Failed to create S3 client")
		claim.Status.Phase = "Error"
		r.Status().Update(ctx, claim)
		return ctrl.Result{}, err
	}

	// Determine bucket name
	bucketName := r.determineBucketName(claim)
	
	// Store bucket name and retain policy in annotations for deletion handling
	if claim.Annotations == nil {
		claim.Annotations = make(map[string]string)
	}
	claim.Annotations[annotationBucketName] = bucketName
	claim.Annotations[annotationRetainPolicy] = string(claim.Spec.RetainPolicy)
	if err := r.Update(ctx, claim); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure bucket exists
	err = ensureBucket(ctx, s3Client, bucketName, region)
	if err != nil {
		log.Error(err, "Failed to ensure bucket", "bucket", bucketName)
		claim.Status.Phase = "Error"
		r.Status().Update(ctx, claim)
		return ctrl.Result{}, err
	}

	// Create Secret for bucket access
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-bucket-secret", claim.Name),
			Namespace: claim.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"AWS_ACCESS_KEY_ID":     accessKey,
			"AWS_SECRET_ACCESS_KEY": secretKey,
			"BUCKET_NAME":           bucketName,
			"BUCKET_HOST":           endpoint,
			"BUCKET_REGION":         region,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(claim, secret, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Create/Update Secret
	if err := upsertSecret(ctx, r.Client, secret); err != nil {
		log.Error(err, "Failed to create/update secret")
		return ctrl.Result{}, err
	}

	// Create ConfigMap for bucket configuration
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-bucket-config", claim.Name),
			Namespace: claim.Namespace,
		},
		Data: map[string]string{
			"BUCKET_NAME":   bucketName,
			"BUCKET_HOST":   endpoint,
			"BUCKET_REGION": region,
			"BUCKET_PORT":   "443",
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(claim, configMap, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Create/Update ConfigMap
	if err := upsertConfigMap(ctx, r.Client, configMap); err != nil {
		log.Error(err, "Failed to create/update configmap")
		return ctrl.Result{}, err
	}

	// Update status
	claim.Status.Phase = "Bound"
	claim.Status.BucketName = bucketName
	claim.Status.SecretRef = secret.Name
	claim.Status.ConfigMapRef = configMap.Name

	if err := r.Status().Update(ctx, claim); err != nil {
		log.Error(err, "Failed to update QuObjectBucketClaim status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled QuObjectBucketClaim", "bucket", bucketName)
	return ctrl.Result{}, nil
}

// determineBucketName determines the bucket name based on the spec
func (r *QuObjectBucketClaimReconciler) determineBucketName(claim *quv1.QuObjectBucketClaim) string {
	// If explicit bucket name is provided, use it
	if claim.Spec.BucketName != "" {
		return claim.Spec.BucketName
	}

	// If already have a bucket name in status, reuse it (for idempotency)
	if claim.Status.BucketName != "" {
		return claim.Status.BucketName
	}

	// Generate a new bucket name with random suffix
	if claim.Spec.GenerateBucketName != "" {
		suffix := generateRandomString(5)
		return fmt.Sprintf("%s-%s", claim.Spec.GenerateBucketName, suffix)
	}

	// Fallback: use namespace-name pattern with random suffix
	suffix := generateRandomString(5)
	return fmt.Sprintf("%s-%s-%s", claim.Namespace, claim.Name, suffix)
}

// generateRandomString generates a random alphanumeric string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// handleDeletion handles the deletion of the QuObjectBucketClaim
func (r *QuObjectBucketClaimReconciler) handleDeletion(
	ctx context.Context,
	claim *quv1.QuObjectBucketClaim,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(claim, finalizerName) {
		log.Info("Processing QuObjectBucketClaim deletion", 
			"Name", claim.Name, 
			"RetainPolicy", claim.Spec.RetainPolicy)

		// Check retain policy
		if claim.Spec.RetainPolicy == quv1.RetainPolicyDelete {
			// Delete the bucket if policy is Delete
			bucketName := claim.Annotations[annotationBucketName]
			if bucketName == "" {
				bucketName = claim.Status.BucketName
			}

			if bucketName != "" {
				log.Info("Deleting bucket per retain policy", "bucket", bucketName)
				
				// Get S3 credentials
				credSecret := &corev1.Secret{}
				err := r.Get(ctx, types.NamespacedName{
					Name:      "s3-credentials",
					Namespace: controllerNS,
				}, credSecret)
				if err != nil {
					log.Error(err, "Failed to get S3 credentials for bucket deletion")
					// Continue with finalizer removal even if we can't delete the bucket
				} else {
					// Create S3 client and delete bucket
					endpoint := string(credSecret.Data["endpoint"])
					region := string(credSecret.Data["region"])
					accessKey := string(credSecret.Data["accessKey"])
					secretKey := string(credSecret.Data["secretKey"])
					
					s3Client, err := newS3Client(endpoint, region, accessKey, secretKey, true, true)
					if err == nil {
						if err := deleteBucket(ctx, s3Client, bucketName); err != nil {
							log.Error(err, "Failed to delete bucket", "bucket", bucketName)
							// Continue with finalizer removal
						} else {
							log.Info("Successfully deleted bucket", "bucket", bucketName)
						}
					}
				}
			}
		} else {
			// Retain policy - keep the bucket
			log.Info("Retaining bucket per retain policy", 
				"bucket", claim.Status.BucketName)
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(claim, finalizerName)
		if err := r.Update(ctx, claim); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deleteBucket deletes an S3 bucket (must be empty)
func deleteBucket(ctx context.Context, s3c *s3.Client, bucket string) error {
	// First, delete all objects in the bucket
	// List objects
	listResp, err := s3c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// Delete each object
	for _, obj := range listResp.Contents {
		_, err := s3c.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    obj.Key,
		})
		if err != nil {
			return fmt.Errorf("failed to delete object %s: %w", *obj.Key, err)
		}
	}

	// Now delete the bucket
	_, err = s3c.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// Check if bucket doesn't exist (already deleted)
		if strings.Contains(strings.ToLower(err.Error()), "nosuchbucket") {
			return nil
		}
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *QuObjectBucketClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quv1.QuObjectBucketClaim{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

// Helper functions

func newS3Client(
	endpoint, region, accessKey, secretKey string,
	useSSL, forcePath bool,
) (*s3.Client, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}}
	hclient := &http.Client{Transport: tr}

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithHTTPClient(hclient),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = forcePath
	}), nil
}

func ensureBucket(ctx context.Context, s3c *s3.Client, bucket, region string) error {
	_, err := s3c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}

	_, err = s3c.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		},
	})
	if err != nil {
		l := strings.ToLower(err.Error())
		if !strings.Contains(l, "bucketalreadyownedbyyou") &&
			!strings.Contains(l, "bucketalreadyexists") {
			return err
		}
	}
	return nil
}

func upsertSecret(ctx context.Context, c client.Client, s *corev1.Secret) error {
	var existing corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		return c.Create(ctx, s)
	} else if err != nil {
		return err
	}
	existing.StringData = s.StringData
	existing.Type = s.Type
	return c.Update(ctx, &existing)
}

func upsertConfigMap(ctx context.Context, c client.Client, m *corev1.ConfigMap) error {
	var existing corev1.ConfigMap
	err := c.Get(ctx, types.NamespacedName{Name: m.Name, Namespace: m.Namespace}, &existing)
	if apierrors.IsNotFound(err) {
		return c.Create(ctx, m)
	} else if err != nil {
		return err
	}
	existing.Data = m.Data
	return c.Update(ctx, &existing)
}
