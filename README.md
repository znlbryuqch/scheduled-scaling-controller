# scheduled-scaling-controller

Scale a Deployment up for known busy windows (business hours, batch slots,
release trains) without hand-editing replica counts the night before.

The controller watches `ScalePlan` objects. Each plan names one Deployment and a
list of cron windows; while a window is open the controller writes the window
replicas to the target's `scale` subresource, and when the window closes it
falls back to `spec.defaultReplicas`.

## Why

Horizontal autoscalers react to load. Some load is predictable in advance, and
reacting to it is already too late: a queue drain at 06:00 or a nightly import
needs capacity at 05:55. A schedule is the honest expression of that.

## Architecture

```text
          ScalePlan (cron windows)            Deployment/scale
                    |                                ^
                    v                                |
        +----------------------+   writes replicas   |
        |  ScalePlanReconciler |---------------------+
        +----------------------+
                    |
                    v
        status.activeWindow, conditions, requeue at the next boundary
```

<!-- diagram placeholder: reconciler loop, window evaluation and status write -->

The reconciler requeues itself at the next window boundary instead of polling,
and it never touches the Deployment outside the `scale` subresource, so
autoscaler integrations keep working.

## CRD

```yaml
apiVersion: scaling.znlbryuqch.io/v1alpha1
kind: ScalePlan
metadata:
  name: checkout-batch
spec:
  targetRef:
    kind: Deployment
    name: checkout
  defaultReplicas: 2
  windows:
    # timeZone is required for windows that must follow local office hours
    - name: nightly-import
      schedule: "55 5 * * *"
      timeZone: Europe/Prague
      duration: 2h
      replicas: 8
    - name: lunch-rush
      schedule: "30 11 * * 1-5"
      timeZone: Europe/Prague
      duration: 90m
      replicas: 6
```

| Field | Meaning |
| --- | --- |
| `spec.targetRef` | Deployment in the plan namespace; only `kind: Deployment` is served |
| `spec.windows[]` | cron expression, IANA `timeZone`, `duration`, `replicas`; the last open window wins |
| `spec.defaultReplicas` | replicas after a window closes; unset leaves the target untouched |
| `status.activeWindow` | window last applied, empty when none is open |
| `status.conditions[Ready]` | `InvalidSchedule`, `TargetMissing`, or `Scaled` |

## Install

```sh
kubectl apply -f config/crd/bases/scaling.znlbryuqch.io_scaleplans.yaml
kubectl apply -f config/rbac/role.yaml      # skip if you run as cluster-admin

go mod download
go run .                                    # or: go build -o manager . && ./manager
```

The controller needs a kubeconfig in scope; leader election is on, so several
replicas are safe.

## Usage

```sh
kubectl apply -f config/samples/scaling_v1alpha1_scaleplan.yaml
kubectl get scaleplans -w          # ACTIVE, REPLICAS
kubectl describe scaleplan checkout-batch
```

Deleting a plan leaves the target at whatever replica count was last applied.

## Layout

```text
api/v1alpha1/            ScalePlan types, group registration, deepcopy
internal/controller/     reconcile loop, window evaluation, requeue logic
main.go                  manager wiring, health and metrics endpoints
config/                  CRD, RBAC, sample manifests
```

## Limitations

- One window set per target; two plans on the same Deployment fight.
- Windows are computed from cron fires, so a missed fire while the controller
  is down is not backfilled beyond the last seven days.
- Replica counts written by the plan are not merged with HPA decisions; run one
  or the other per Deployment.

## License

MIT, see [LICENSE](LICENSE).
