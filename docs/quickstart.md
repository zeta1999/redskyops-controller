# Quick Start

This is brief guide to get you up and running with Red Sky Ops as quickly as possible.

## Prerequisites

You must have a Kubernetes cluster. Additionally, you will need a local configured copy of `kubectl`. The Red Sky Ops Tool will use the same configuration as `kubectl` (usually `$HOME/.kube/config`) to connect to your cluster.

A local install of [Kustomize](https://github.com/kubernetes-sigs/kustomize/releases) (v3.1.0+) is required to build the objects for your cluster.

If you are planing to create the simple experiment from this guide, a [minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/) cluster is preferred.

## Install the Red Sky Ops Tool

[Download](https://github.com/redskyops/redskyops-controller/releases) and install the `redskyctl` binary for your platform. You will need to rename the downloaded file and mark it as executable.

For more details, see [the installation guide](install.md).

## Initialize the Red Sky Ops Manager

Once you have the Red Sky Ops Tool you can initialize the manager in your cluster:

```sh
$ redskyctl init
```

## Create a Simple Experiment

Generally you will want to write your own experiments to run trials on your own applications. For the purposes of this guide we can use the simple example found in the `redskyops-recipes` [repository on GitHub](https://github.com/redskyops/redskyops-recipes/tree/master/simple):

```sh
$ kustomize build github.com/redskyops/redskyops-recipes//simple | kubectl apply -f -
```

## Run a Trial

With your experiment created, you can be begin running trials by suggesting parameter assignments locally. Each trial will create one or more Kubernetes jobs and will conclude by collecting a small number of metric values indicative of the performance for the trial.

To interactively create a new trial for the example experiment, run:

```sh
$ redskyctl suggest --interactive simple
```

You will be prompted to enter a value for each parameter in the experiment and a new trial will be created. You can monitor the progress using `kubectl`:

```sh
$ kubectl get trials
```

When running interactive trials in a single namespace, be sure only trial is active at a time.

## Removing the Experiment

To clean up the data from your experiment, simply delete the experiment. The delete will cascade to the associated trials and other Kubernetes objects:

```sh
$ kubectl delete experiment simple
```

## Next Steps

Congratulations! You just ran your first experiment. You can move on to a more [advanced tutorial](tutorial.md) or browse the rest of the documentation to learn more about the Red Sky Ops Kubernetes experimentation product.
