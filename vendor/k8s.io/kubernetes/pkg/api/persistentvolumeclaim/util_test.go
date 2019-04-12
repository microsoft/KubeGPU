/*
Copyright 2017 The Kubernetes Authors.

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

package persistentvolumeclaim

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilfeaturetesting "k8s.io/apiserver/pkg/util/feature/testing"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/features"
)

func TestDropAlphaPVCVolumeMode(t *testing.T) {
	vmode := core.PersistentVolumeFilesystem

	pvcWithoutVolumeMode := func() *core.PersistentVolumeClaim {
		return &core.PersistentVolumeClaim{
			Spec: core.PersistentVolumeClaimSpec{
				VolumeMode: nil,
			},
		}
	}
	pvcWithVolumeMode := func() *core.PersistentVolumeClaim {
		return &core.PersistentVolumeClaim{
			Spec: core.PersistentVolumeClaimSpec{
				VolumeMode: &vmode,
			},
		}
	}

	pvcInfo := []struct {
		description   string
		hasVolumeMode bool
		pvc           func() *core.PersistentVolumeClaim
	}{
		{
			description:   "pvc without VolumeMode",
			hasVolumeMode: false,
			pvc:           pvcWithoutVolumeMode,
		},
		{
			description:   "pvc with Filesystem VolumeMode",
			hasVolumeMode: true,
			pvc:           pvcWithVolumeMode,
		},
		{
			description:   "is nil",
			hasVolumeMode: false,
			pvc:           func() *core.PersistentVolumeClaim { return nil },
		},
	}

	for _, enabled := range []bool{true, false} {
		for _, oldpvcInfo := range pvcInfo {
			for _, newpvcInfo := range pvcInfo {
				oldpvcHasVolumeMode, oldpvc := oldpvcInfo.hasVolumeMode, oldpvcInfo.pvc()
				newpvcHasVolumeMode, newpvc := newpvcInfo.hasVolumeMode, newpvcInfo.pvc()
				if newpvc == nil {
					continue
				}

				t.Run(fmt.Sprintf("feature enabled=%v, old pvc %v, new pvc %v", enabled, oldpvcInfo.description, newpvcInfo.description), func(t *testing.T) {
					defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.BlockVolume, enabled)()

					var oldpvcSpec *core.PersistentVolumeClaimSpec
					if oldpvc != nil {
						oldpvcSpec = &oldpvc.Spec
					}
					DropDisabledFields(&newpvc.Spec, oldpvcSpec)

					// old pvc should never be changed
					if !reflect.DeepEqual(oldpvc, oldpvcInfo.pvc()) {
						t.Errorf("old pvc changed: %v", diff.ObjectReflectDiff(oldpvc, oldpvcInfo.pvc()))
					}

					switch {
					case enabled || oldpvcHasVolumeMode:
						// new pvc should not be changed if the feature is enabled, or if the old pvc had BlockVolume
						if !reflect.DeepEqual(newpvc, newpvcInfo.pvc()) {
							t.Errorf("new pvc changed: %v", diff.ObjectReflectDiff(newpvc, newpvcInfo.pvc()))
						}
					case newpvcHasVolumeMode:
						// new pvc should be changed
						if reflect.DeepEqual(newpvc, newpvcInfo.pvc()) {
							t.Errorf("new pvc was not changed")
						}
						// new pvc should not have BlockVolume
						if !reflect.DeepEqual(newpvc, pvcWithoutVolumeMode()) {
							t.Errorf("new pvc had pvcBlockVolume: %v", diff.ObjectReflectDiff(newpvc, pvcWithoutVolumeMode()))
						}
					default:
						// new pvc should not need to be changed
						if !reflect.DeepEqual(newpvc, newpvcInfo.pvc()) {
							t.Errorf("new pvc changed: %v", diff.ObjectReflectDiff(newpvc, newpvcInfo.pvc()))
						}
					}
				})
			}
		}
	}
}

func TestDropDisabledDataSource(t *testing.T) {
	pvcWithoutDataSource := func() *core.PersistentVolumeClaim {
		return &core.PersistentVolumeClaim{
			Spec: core.PersistentVolumeClaimSpec{
				DataSource: nil,
			},
		}
	}
	apiGroup := "snapshot.storage.k8s.io"
	pvcWithDataSource := func() *core.PersistentVolumeClaim {
		return &core.PersistentVolumeClaim{
			Spec: core.PersistentVolumeClaimSpec{
				DataSource: &core.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     "VolumeSnapshot",
					Name:     "test_snapshot",
				},
			},
		}
	}

	pvcInfo := []struct {
		description   string
		hasDataSource bool
		pvc           func() *core.PersistentVolumeClaim
	}{
		{
			description:   "pvc without DataSource",
			hasDataSource: false,
			pvc:           pvcWithoutDataSource,
		},
		{
			description:   "pvc with DataSource",
			hasDataSource: true,
			pvc:           pvcWithDataSource,
		},
		{
			description:   "is nil",
			hasDataSource: false,
			pvc:           func() *core.PersistentVolumeClaim { return nil },
		},
	}

	for _, enabled := range []bool{true, false} {
		for _, oldpvcInfo := range pvcInfo {
			for _, newpvcInfo := range pvcInfo {
				oldPvcHasDataSource, oldpvc := oldpvcInfo.hasDataSource, oldpvcInfo.pvc()
				newPvcHasDataSource, newpvc := newpvcInfo.hasDataSource, newpvcInfo.pvc()
				if newpvc == nil {
					continue
				}

				t.Run(fmt.Sprintf("feature enabled=%v, old pvc %v, new pvc %v", enabled, oldpvcInfo.description, newpvcInfo.description), func(t *testing.T) {
					defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.VolumeSnapshotDataSource, enabled)()

					var oldpvcSpec *core.PersistentVolumeClaimSpec
					if oldpvc != nil {
						oldpvcSpec = &oldpvc.Spec
					}
					DropDisabledFields(&newpvc.Spec, oldpvcSpec)

					// old pvc should never be changed
					if !reflect.DeepEqual(oldpvc, oldpvcInfo.pvc()) {
						t.Errorf("old pvc changed: %v", diff.ObjectReflectDiff(oldpvc, oldpvcInfo.pvc()))
					}

					switch {
					case enabled || oldPvcHasDataSource:
						// new pvc should not be changed if the feature is enabled, or if the old pvc had DataSource
						if !reflect.DeepEqual(newpvc, newpvcInfo.pvc()) {
							t.Errorf("new pvc changed: %v", diff.ObjectReflectDiff(newpvc, newpvcInfo.pvc()))
						}
					case newPvcHasDataSource:
						// new pvc should be changed
						if reflect.DeepEqual(newpvc, newpvcInfo.pvc()) {
							t.Errorf("new pvc was not changed")
						}
						// new pvc should not have DataSource
						if !reflect.DeepEqual(newpvc, pvcWithoutDataSource()) {
							t.Errorf("new pvc had DataSource: %v", diff.ObjectReflectDiff(newpvc, pvcWithoutDataSource()))
						}
					default:
						// new pvc should not need to be changed
						if !reflect.DeepEqual(newpvc, newpvcInfo.pvc()) {
							t.Errorf("new pvc changed: %v", diff.ObjectReflectDiff(newpvc, newpvcInfo.pvc()))
						}
					}
				})
			}
		}
	}
}
