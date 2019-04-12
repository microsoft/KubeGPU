/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1alpha1 "k8s.io/api/node/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	node "k8s.io/kubernetes/pkg/apis/node"
)

func TestRuntimeClassConversion(t *testing.T) {
	const (
		name    = "puppy"
		handler = "heidi"
	)
	internalRC := node.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Handler:    handler,
	}
	v1alpha1RC := v1alpha1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.RuntimeClassSpec{
			RuntimeHandler: handler,
		},
	}

	convertedInternal := node.RuntimeClass{}
	require.NoError(t,
		Convert_v1alpha1_RuntimeClass_To_node_RuntimeClass(&v1alpha1RC, &convertedInternal, nil))
	assert.Equal(t, internalRC, convertedInternal)

	convertedV1alpha1 := v1alpha1.RuntimeClass{}
	require.NoError(t,
		Convert_node_RuntimeClass_To_v1alpha1_RuntimeClass(&internalRC, &convertedV1alpha1, nil))
	assert.Equal(t, v1alpha1RC, convertedV1alpha1)
}
