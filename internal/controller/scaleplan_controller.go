package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	scalev1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	scalingv1alpha1 "github.com/znlbryuqch/scheduled-scaling-controller/api/v1alpha1"
	"github.com/znlbryuqch/scheduled-scaling-controller/internal/schedule"
)

// fallbackRequeue bounds the sleep between window boundaries.
const fallbackRequeue = 10 * time.Minute

// ScalePlanReconciler applies the replica counts of open windows to the target
// Deployment. It owns no child objects; it only writes the scale subresource
// and the plan status.
type ScalePlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Now is injectable for tests.
	Now func() time.Time
}

// +kubebuilder:rbac:groups=scaling.znlbryuqch.io,resources=scaleplans,verbs=get;list;watch
// +kubebuilder:rbac:groups=scaling.znlbryuqch.io,resources=scaleplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments/scale,verbs=get;update;patch

// Reconcile evaluates the open windows of one ScalePlan and scales its target.
func (r *ScalePlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var plan scalingv1alpha1.ScalePlan
	if err := r.Get(ctx, req.NamespacedName, &plan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now := time.Now
	if r.Now != nil {
		now = r.Now
	}

	windows, err := schedule.Compile(toScheduleWindows(plan.Spec.Windows))
	if err != nil {
		logger.Info("invalid plan", "reason", err.Error())
		r.setCondition(&plan, metav1.ConditionFalse, "InvalidSchedule", err.Error())
		if updateErr := r.Status().Update(ctx, &plan); updateErr != nil {
			logger.Error(updateErr, "updating invalid plan status")
		}
		// A bad expression only changes on spec edits, so do not hot loop.
		return ctrl.Result{}, nil
	}

	window, replicas := windows.Desired(now())
	if plan.Spec.DefaultReplicas != nil && window == "" {
		replicas = *plan.Spec.DefaultReplicas
	}

	target := types.NamespacedName{Namespace: plan.Namespace, Name: plan.Spec.TargetRef.Name}
	var deploy appsv1.Deployment
	if err := r.Get(ctx, target, &deploy); err != nil {
		if apierrors.IsNotFound(err) {
			r.setCondition(&plan, metav1.ConditionFalse, "TargetMissing",
				fmt.Sprintf("deployment %s not found", target))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, &plan)
		}
		return ctrl.Result{}, err
	}

	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas != replicas {
		scale := &scalev1.Scale{ObjectMeta: metav1.ObjectMeta{Name: deploy.Name, Namespace: deploy.Namespace}}
		scale.Spec.Replicas = replicas
		if err := r.SubResource("scale").Update(ctx, &deploy, client.WithSubResourceBody(scale)); err != nil {
			return ctrl.Result{}, fmt.Errorf("scaling %s: %w", target, err)
		}
		logger.Info("scaled workload", "deployment", target.String(), "replicas", replicas, "window", window)
	}

	plan.Status.ActiveWindow = window
	plan.Status.Replicas = replicas
	plan.Status.ObservedGeneration = plan.Generation
	r.setCondition(&plan, metav1.ConditionTrue, "Scaled", "target replica count matches the plan")
	if err := r.Status().Update(ctx, &plan); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: windows.NextBoundary(now(), fallbackRequeue)}, nil
}

// toScheduleWindows narrows the API type to the evaluator input.
func toScheduleWindows(in []scalingv1alpha1.ScaleWindow) []schedule.Window {
	out := make([]schedule.Window, 0, len(in))
	for _, w := range in {
		out = append(out, schedule.Window{
			Name:     w.Name,
			Schedule: w.Schedule,
			TimeZone: w.TimeZone,
			Duration: w.Duration.Duration,
			Replicas: w.Replicas,
		})
	}
	return out
}

func (r *ScalePlanReconciler) setCondition(plan *scalingv1alpha1.ScalePlan, status metav1.ConditionStatus, reason, msg string) {
	meta.SetStatusCondition(&plan.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: plan.Generation,
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager registers the reconciler for ScalePlan resources.
func (r *ScalePlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scalingv1alpha1.ScalePlan{}).
		Complete(r)
}
