# GKE Authentication Plugin

This plugin provides a standalone way to generate an ExecCredential for use by k8s.io/client-go applications.

Google already provides a [gke-cloud-auth-plugin](https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke); however, that plugin depends on the gcloud CLI, which is written in Python. This dependency graph is Large if you want to authenticate and interact with a GKE cluster from a go application.

The plugin is for use outside of a cluster; when running in the cluster, mount a service account and use that token to interact with the Kubernetes API.


