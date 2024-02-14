# turtles-etcd-restore

## Setting up the environment

1. Deploy rancher and rancher turtles extension using your preferred method.
2. Deploy RKE2 provider:
```bash
kubectl apply -f examples/rke2-provider.yaml
```
3. Deploy AWS provider:
```bash
export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)
envsubst < examples/aws-provider.yaml | kubectl apply -f -
```
4. Deploy RKE2 Cluster in AWS:
```bash
export CLUSTER_NAME="rke2-aws-cluster"
export SSH_KEY="yourkeyname"
export AWS_REGION="us-west-2"

envsubst < examples/rke2-aws-cluster.yaml | kubectl apply -f -
```