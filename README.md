![GitHub](https://img.shields.io/github/license/josecastillolema/krkn-operator)
![GitHub language count](https://img.shields.io/github/languages/count/josecastillolema/krkn-operator)
![GitHub top language](https://img.shields.io/github/languages/top/josecastillolema/krkn-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/josecastillolema/krkn-operator)](https://goreportcard.com/report/github.com/josecastillolema/krkn-operator)

# krkn-operator
The krkn-operator provides a way to run [krkn](https://github.com/redhat-chaos/krkn) chaos test scenarios in various [predefined deployment configurations](https://github.com/redhat-chaos/krkn-hub).

## Use
Create a benchmark CRD, i.e.:
```yaml
apiVersion: perf.chaos.io/v1
kind: Benchmark
metadata:
  name: benchmark-sample
spec:
  scenario: pod-scenarios
```

```
$ kubectl apply -f config/samples/perf_v1_benchmark.yaml
benchmark.perf.chaos.io/benchmark-sample created

$ kubectl get benchmark
NAME               AGE
benchmark-sample   7s
```

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [kind](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.

**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Deploy the controller to the cluster:

```sh
make deploy IMG=quay.io/jcastillolema/krkn-operator:0.0.1
```

2. Verify that the krkn-operator is up and running:
```
$ k get po -n krkn-operator-system
NAME                                               READY   STATUS    RESTARTS   AGE
krkn-operator-controller-manager-b8585c7cb-88xjh   2/2     Running   0          18m
```

3. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=quay.io/jcastillolema/krkn-operator:0.0.1
```



### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing


### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

