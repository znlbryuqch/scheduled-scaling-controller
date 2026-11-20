// Package v1alpha1 contains the API types for the scaling.znlbryuqch.io group.
// +kubebuilder:object:generate=true
// +groupName=scaling.znlbryuqch.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group and version served by this API package.
	GroupVersion = schema.GroupVersion{Group: "scaling.znlbryuqch.io", Version: "v1alpha1"}

	// SchemeBuilder registers the types of this package with a scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types of this package to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
