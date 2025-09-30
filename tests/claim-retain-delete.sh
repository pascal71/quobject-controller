# Create with Delete policy
kubectl apply -f - <<EOF
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: delete-test
spec:
  generateBucketName: test
  retainPolicy: Delete
EOF

sleep 300

# Delete it - should delete the bucket
kubectl delete quobjectbucketclaim delete-test

# Create with Retain policy (default)
kubectl apply -f - <<EOF
apiVersion: quobject.io/v1alpha1
kind: QuObjectBucketClaim
metadata:
  name: retain-test
spec:
  generateBucketName: test
  retainPolicy: Retain
EOF
sleep 300

# Delete it - bucket should remain
kubectl delete quobjectbucketclaim retain-test
