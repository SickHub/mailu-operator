package controller

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	FinalizerName = "operator.mailu.io/finalizer"
)

// How to make this cleaner/better?
func GetReadyCondition(cond string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    cond,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// WithStatus() ?
