# GKE Authentication Plugin

This plugin provides a standalone way to generate an ExecCredential for use by k8s.io/client-go applications.

Google already provides a [gke-gcloud-auth-plugin](https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke); however, that plugin depends on the gcloud CLI, which is written in Python. This dependency graph is Large if you want to authenticate and interact with a GKE cluster from a go application.

The plugin is for use outside of a cluster; when running in the cluster, mount a service account and use that token to interact with the Kubernetes API.

## Build

```shell
make
```

Or with Docker
```shell
docker build -f Dockerfile.dev -t gke-auth-plugin-dev .

docker run -it --rm --name gke-auth-plugin-dev-container -v ${PWD}:/home/nonroot gke-auth-plugin-dev
```

## Run

```shell
# generate ExecCredential
bin/gke-auth-plugin

# version
bin/gke-auth-plugin version
```

You can straight up replace the gke-gcloud-auth-plugin with this binary, or place on your path and update your kubeconfig exec command to run gke-auth-plugin.

### Example Exec Section of Kubeconfig

```yaml
users:
- name: user_id
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-auth-plugin
      provideClusterInfo: true
      interactiveMode: Never
```
## TODO

- Add unit tests