
# Bigtable Adaptor Deployment on GKE

Deploying the Bigtable Adaptor on Google Kubernetes Engine (GKE) facilitates seamless communication between your application and Google Cloud Bigtable. Below is an overview of the major steps involved in the deployment process.

## Prerequisites

Ensure you have the following before getting started:

1. **GCP Editor Role Permissions:**
   - Obtain the GCP Editor role permissions to manage resources on Google Cloud Platform.
2. **kubectl CLI:**
   - Install `kubectl` CLI for Kubernetes cluster deployment. Follow the [Kubernetes Documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for installation instructions.
3. **Docker:**
   - Build docker image and push to Artifact Registry.

## Usage Instructions:

### Step 1: Build and Push Docker Image

Clone the repository:

```bash
git clone git@github.com:ollionorg/cassandra-to-bigtable-proxy.git
```

Prepare Google Cloud CLI Config:

```bash
export PROJECT_ID=<project_id>
export REGION=<region>
gcloud auth application-default login --project $PROJECT_ID
gcloud config set project $PROJECT_ID
```

Retrieve values from the Terraform output and export them as environment variables:

```bash
terraform output
```

```bash
artifact-registry = "bigtable-adaptor-docker-dev"
gke_cluster = "bigtable-adaptor-gke-dev"
kubectl_proxy_vm = "bigtable-proxy-vm-dev"
server_account = "bigtable-adaptor-proxy-sa-dev@<project_id>.iam.gserviceaccount.com"
subnet_name_pub = "bigtable-adaptor-pub-subnetwork-dev"
subnet_name_pvt = "bigtable-adaptor-pvt-subnetwork-dev"
vpc_name = "bigtable-adaptor-vpc-dev"
---------------
export SERVICE_ACCOUNT="bigtable-adaptor-proxy-sa-dev@<project_id>.iam.gserviceaccount.com"
export ARTIFACT_REGISTRY="bigtable-adaptor-docker-dev"
export APP_NAME="bigtable-adaptor"
export PROXY_VM_NAME="bigtable-proxy-vm-dev"
export GKE_CLUSTER="bigtable-adaptor-gke-dev"
```

Configure Google Cloud Artifact Registry:

```bash
gcloud auth configure-docker $REGION-docker.pkg.dev
```

Build and Push Docker Image:

```bash
cd cassandra-to-bigtable-proxy
docker build --platform linux/amd64 -t $REGION-docker.pkg.dev/$PROJECT_ID/$ARTIFACT_REGISTRY/"$APP_NAME":tag1 .
docker push $REGION-docker.pkg.dev/$PROJECT_ID/$ARTIFACT_REGISTRY/"$APP_NAME":tag1
```

### Step 2: Create Secrets and Deploy Bigtable Adaptor

Create a JSON Key for Google Cloud Service Account:

```bash
gcloud iam service-accounts keys create --iam-account "$SERVICE_ACCOUNT@$PROJECT_ID.iam.gserviceaccount.com"~/Downloads/bigtable-adaptor-service-account.json
```

Modify the YAML file: like `GOOGLE_APPLICATION_CREDENTIALS`

```bash
vi bigtable-deployment.yaml
```

```
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    run: bigtable-adaptor-app
  name: bigtable-adaptor-app
  namespace: deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      run: bigtable-adaptor-app
  template: # Pod template
    metadata:
      labels:
        run: bigtable-adaptor-app # Labels Pods from this Deployment
    spec: # Pod specification; each Pod created by this Deployment has this specification
      containers:
      - name: bigtable-adaptor-app
        image: REGION-docker.pkg.dev/GCP_PRO_ID/ARTIFACT_REGISTRY/IMA_NAME:IMG_TAG
        resources:
          requests:
            memory: "1Gi"
            cpu: "1"
          limits:
            memory: "1Gi"
            cpu: "1"
        env:
          - name: GOOGLE_APPLICATION_CREDENTIALS
            value: "/var/run/secret/cloud.google.com/bigtable-adaptor-service-account.json"
          - name: GCP_PROJECT_ID
            valueFrom:
              secretKeyRef:
                name: app-secret-env-variable
                key: gcp_project_id
        volumeMounts:
          - name: "service-account"
            mountPath: "/var/run/secret/cloud.google.com"
        command: [ "./cassandra-to-bigtable-proxy" ]
        ports:
        - containerPort: 9042
          protocol: TCP
      volumes:
        - name: "service-account"
          secret:
            secretName: "app-secret-env-variable"
```

Make the necessary updates in the file and save the changes.

### Step 3: Establish a Connection to the GKE Private Cluster

Use the `kubectl-proxy-vm-dev` to connect to the Private GKE Cluster:

```bash
gcloud container clusters get-credentials $GKE_CLUSTER --region $REGION --project $PROJECT_ID
gcloud compute ssh $PROXY_VM_NAME --tunnel-through-iap --project=$PROJECT_ID --zone=${REGION}-a --ssh-flag="-4 -L8888:localhost:8888 -N -q -f"
export HTTPS_PROXY=localhost:8888
kubectl get pod -A
```

Create Secrets:

```bash
# Create namespace
kubectl create namespace deployment
#This will create a secret in Kubernetes called app-secret-sa in the deployment namespace.
kubectl create secret generic app-secret-env-variable \
  -n deployment \
  --from-literal=gcp_project_id="<project_id>" \
  --from-file ./bigtable-adaptor-service-account.json
#You can list all the secrets in Kubernetes (that you can see) via:
kubectl get secret -n deployment
```

### Step 4: Deploy Bigtable Adaptor

Once GKE cluster access is obtained, proceed with the deployment:

```bash
cd k8/
kubectl apply -f bigtable-deployment.yaml
```

Unset IAP Tunneling after you finish using GKE:

```bash
gcloud config unset proxy/type
gcloud config unset proxy/address
gcloud config unset proxy/port
unset HTTPS_PROXY
unset HTTP_PROXY
```

### Step 5: Deploy K8s Loadbalancer
An internal load balancer in Kubernetes on GKE is used to expose Bigtable Adaptor service privately within your Virtual Private Cloud (VPC), without assigning an external IP accessible from the internet.

```bash
vi loadbalancer-service.yaml
```

```
apiVersion: v1
kind: Service
metadata:
  name: bigtable-adaptor-svc # Name of Service
  annotations:
    cloud.google.com/load-balancer-type: Internal
  namespace: deployment
spec: # Service's specification
  type: LoadBalancer
  selector:
    run: bigtable-adaptor-app
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9042
```

```bash
kubectl apply -f loadbalancer-service.yaml
```

## Note

To update the config.yaml of the application, follow these steps:

    1. Make the necessary changes to the config.yaml file locally.
    2. Upload the updated config.yaml file to the bigtable-fuse-csi-proxy-ad bucket.

Whenever the GitHub Action runs next time, it will retrieve the updated config.yaml file from the GCS bucket.
