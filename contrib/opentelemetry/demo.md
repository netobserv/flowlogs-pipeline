
# Demo of flowlogs-pipeline with Opentelemetry traces and Jaeger

We provide here instructions how to bring up a simple demo that uses the flowlogs-pipeline opentelemetry capability.
We send trace data to the opentelemetry collector, which is then forwarded to jaeger to be presented in its UI.

We assume we have a kubernetes cluster environment.
This may be a real cluster such as Openshift cluster or a simulated cluster such as minikube.

We install jaeger and the opentelemetry collector using operators.
The operators require the existance of cert manager, so we first install cert manager.

```
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.2/cert-manager.yaml
```

Wait for all pods in namespace cert-manager to be running.

Install jaeger operator: See https://www.jaegertracing.io/docs/1.52/operator/

```
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.52.0/jaeger-operator.yaml -n observability
```

Wait for operator to be ready

Install jaeger instance.
In directory githum.com/netobserv/flowlogs-pipeline/contrib/opentelemetry:

```
kubectl create namespace jaeger
kubectl apply -f ./jaeger.yaml -n jaeger
```

Install opentelemetry operator: See https://opentelemetry.io/docs/kubernetes/operator/

```
kubectl apply -f https://github.com/open-telemetry/opentelemetry-operator/releases/latest/download/opentelemetry-operator.yaml
```

Wait for operator to be ready.

Install opentelemetry collector instance.
In directory githum.com/netobserv/flowlogs-pipeline/contrib/opentelemetry:

```
kubectl create namespace otlp
kubectl apply -f ./collector.yaml -n otlp
```

Install ebpf and flowlogs-pipeline.
In directory githum.com/netobserv/flowlogs-pipeline/contrib/opentelemetry:

```
kubectl create namespace netobserv
kubectl apply -f ./perms.yml # (ignore the warnings)
kubectl apply -f ./flp.yml -n netobserv
```

(Optional) Install some test workload.

```
kubectl create namespace mesh-arena
kubectl apply -f ./mesh-arena.yml -n mesh-arena
```

Access the jaeger UI.
On Openshift, connect to jaeger UI at:

```
oc get route my-jaeger -o jsonpath='{.spec.host}' -n jaeger
```

Then:
```
https://<my-jaeger host address>
```

On Minikube:

```
kubectl port-forward --address 0.0.0.0 svc/my-jaeger-query -n jaeger 16686:16686 2>&1 >/dev/null &
```

Then:
```
http://<localhost>:16686
```


