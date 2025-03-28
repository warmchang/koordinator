/*
Copyright 2022 The Koordinator Authors.

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

package cpuset

import (
	"path/filepath"
	"reflect"
	"testing"

	topov1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	ext "github.com/koordinator-sh/koordinator/apis/extension"
	slov1alpha1 "github.com/koordinator-sh/koordinator/apis/slo/v1alpha1"
	"github.com/koordinator-sh/koordinator/pkg/features"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	koordletutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	"github.com/koordinator-sh/koordinator/pkg/util"
)

func Test_cpusetRule_getContainerCPUSet(t *testing.T) {
	type fields struct {
		kubeletPolicy   string
		sharePools      []ext.CPUSharedPool
		beSharePools    []ext.CPUSharedPool
		systemQOSCPUSet string
	}
	type args struct {
		podAlloc            *ext.ResourceStatus
		containerReq        *protocol.ContainerRequest
		beCPUManagerEnabled bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *string
		wantErr bool
	}{
		{
			name: "get cpuset from bad annotation",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels:     map[string]string{},
					PodAnnotations: map[string]string{
						ext.AnnotationResourceStatus: "bad-alloc-fmt",
					},
					CgroupParent: "burstable/test-pod/test-container",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get cpuset from annotation be share pool",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "1-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "9-15",
					},
				},
				beSharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels: map[string]string{
						ext.LabelPodQoS: string(ext.QoSBE),
					},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
				podAlloc: &ext.ResourceStatus{
					NUMANodeResources: []ext.NUMANodeResource{
						{
							Node: 0,
							Resources: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI),
							},
						},
					},
				},
				beCPUManagerEnabled: true,
			},
			want:    pointer.String("0-7"),
			wantErr: false,
		},
		{
			name: "get cpuset from annotation share pool",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:        protocol.PodMeta{},
					ContainerMeta:  protocol.ContainerMeta{},
					PodLabels:      map[string]string{},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
				podAlloc: &ext.ResourceStatus{
					NUMANodeResources: []ext.NUMANodeResource{
						{
							Node: 0,
							Resources: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI),
							},
						},
					},
				},
			},
			want:    pointer.String("0-7"),
			wantErr: false,
		},
		{
			name: "get all share pools for ls pod",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels: map[string]string{
						ext.LabelPodQoS: string(ext.QoSLS),
					},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
			},
			want:    pointer.String("0-7,8-15"),
			wantErr: false,
		},
		{
			name: "get all share pools for ls pod with no cpu numa allocation",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels: map[string]string{
						ext.LabelPodQoS: string(ext.QoSLS),
					},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
				podAlloc: &ext.ResourceStatus{
					NUMANodeResources: []ext.NUMANodeResource{
						{
							Node: 0,
							Resources: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceHugePagesPrefix + "1Gi": resource.MustParse("2Gi"),
							},
						},
					},
				},
			},
			want:    pointer.String("0-7,8-15"),
			wantErr: false,
		},
		{
			name: "get all share pools for origin burstable pod under none policy",
			fields: fields{
				kubeletPolicy: ext.KubeletCPUManagerPolicyNone,
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:        protocol.PodMeta{},
					ContainerMeta:  protocol.ContainerMeta{},
					PodLabels:      map[string]string{},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
			},
			want:    pointer.String("0-7,8-15"),
			wantErr: false,
		},
		{
			name: "do nothing for origin burstable pod under static policy",
			fields: fields{
				kubeletPolicy: ext.KubeletCPUManagerPolicyStatic,
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:        protocol.PodMeta{},
					ContainerMeta:  protocol.ContainerMeta{},
					PodLabels:      map[string]string{},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "empty string for origin besteffort pod",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:        protocol.PodMeta{},
					ContainerMeta:  protocol.ContainerMeta{},
					PodLabels:      map[string]string{},
					PodAnnotations: map[string]string{},
					CgroupParent:   "besteffort/test-pod/test-container",
				},
			},
			want:    pointer.String(""),
			wantErr: false,
		},
		{
			name: "get cpuset from annotation ls share pool",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "1-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "9-15",
					},
				},
				beSharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels: map[string]string{
						ext.LabelPodQoS: string(ext.QoSLS),
					},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
				podAlloc: &ext.ResourceStatus{
					NUMANodeResources: []ext.NUMANodeResource{
						{
							Node: 1,
							Resources: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI),
							},
						},
					},
				},
			},
			want:    pointer.String("9-15"),
			wantErr: false,
		},
		{
			name: "get cpuset from annotation be share pool",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "1-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "9-15",
					},
				},
				beSharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				beCPUManagerEnabled: true,
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels: map[string]string{
						ext.LabelPodQoS: string(ext.QoSBE),
					},
					PodAnnotations: map[string]string{},
					CgroupParent:   "besteffort/test-pod/test-container",
				},
				podAlloc: &ext.ResourceStatus{
					NUMANodeResources: []ext.NUMANodeResource{
						{
							Node: 1,
							Resources: map[corev1.ResourceName]resource.Quantity{
								ext.BatchCPU: *resource.NewQuantity(2000, resource.DecimalSI),
							},
						},
					},
				},
			},
			want:    pointer.String("8-15"),
			wantErr: false,
		},
		{
			name: "get cpuset from annotation system qos resource",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "4-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "9-15",
					},
				},
				beSharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "4-7",
					},
					{
						Socket: 1,
						Node:   1,
						CPUSet: "8-15",
					},
				},
				systemQOSCPUSet: "0-3",
			},
			args: args{
				containerReq: &protocol.ContainerRequest{
					PodMeta:       protocol.PodMeta{},
					ContainerMeta: protocol.ContainerMeta{},
					PodLabels: map[string]string{
						ext.LabelPodQoS: string(ext.QoSSystem),
					},
					PodAnnotations: map[string]string{},
					CgroupParent:   "burstable/test-pod/test-container",
				},
				podAlloc: &ext.ResourceStatus{
					NUMANodeResources: []ext.NUMANodeResource{
						{
							Node: 1,
							Resources: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceHugePagesPrefix + "1Gi": resource.MustParse("2Gi"),
							},
						},
					},
				},
			},
			want:    pointer.String("0-3"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &cpusetRule{
				kubeletPolicy: ext.KubeletCPUManagerPolicy{
					Policy: tt.fields.kubeletPolicy,
				},
				sharePools:      tt.fields.sharePools,
				beSharePools:    tt.fields.beSharePools,
				systemQOSCPUSet: tt.fields.systemQOSCPUSet,
			}
			if tt.args.podAlloc != nil {
				podAllocJson := util.DumpJSON(tt.args.podAlloc)
				tt.args.containerReq.PodAnnotations[ext.AnnotationResourceStatus] = podAllocJson
			}
			features.DefaultMutableKoordletFeatureGate.SetFromMap(
				map[string]bool{string(features.BECPUManager): tt.args.beCPUManagerEnabled})
			got, err := r.getContainerCPUSet(tt.args.containerReq)
			assert.Equal(t, tt.wantErr, err != nil, err)
			assert.Equal(t, tt.want, got, "cpuset of container should be equal, want %+v, got %+v", util.DumpJSON(tt.want), util.DumpJSON(got))
		})
	}
	// node.koordinator.sh/cpu-shared-pools: '[{"cpuset":"2-7"}]'
	// scheduling.koordinator.sh/resource-status: '{"cpuset":"0-1"}'
}

func Test_cpusetPlugin_parseRuleBadIf(t *testing.T) {
	type fields struct {
		rule *cpusetRule
	}
	type args struct {
		nodeTopo interface{}
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantUpdated bool
		wantRule    *cpusetRule
		wantErr     bool
	}{
		{
			name: "update rule with bad format",
			fields: fields{
				rule: &cpusetRule{
					sharePools: []ext.CPUSharedPool{
						{
							Socket: 0,
							Node:   0,
							CPUSet: "0-7",
						},
						{
							Socket: 1,
							Node:   0,
							CPUSet: "8-15",
						},
					},
				},
			},
			args: args{
				nodeTopo: corev1.Pod{},
			},
			wantUpdated: false,
			wantRule: &cpusetRule{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &cpusetPlugin{
				rule: tt.fields.rule,
			}
			got, err := p.parseRule(tt.args.nodeTopo)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantUpdated {
				t.Errorf("parseRule() got = %v, wantUpdated %v", got, tt.wantUpdated)
			}
			assert.Equal(t, tt.wantRule, p.rule, "after plugin rule parse")
		})
	}
}

func Test_cpusetPlugin_parseRule(t *testing.T) {
	type fields struct {
		rule *cpusetRule
	}
	type args struct {
		nodeTopo     *topov1alpha1.NodeResourceTopology
		cpuPolicy    *ext.KubeletCPUManagerPolicy
		sharePools   []ext.CPUSharedPool
		systemQOSRes *ext.SystemQOSResource
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantUpdated bool
		wantRule    *cpusetRule
		wantErr     bool
	}{
		{
			name: "update rule with bad format",
			fields: fields{
				rule: &cpusetRule{
					sharePools: []ext.CPUSharedPool{
						{
							Socket: 0,
							Node:   0,
							CPUSet: "0-7",
						},
						{
							Socket: 1,
							Node:   0,
							CPUSet: "8-15",
						},
					},
				},
			},
			args: args{
				nodeTopo: &topov1alpha1.NodeResourceTopology{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Annotations: map[string]string{
							ext.AnnotationNodeCPUSharedPools: "bad-fmt",
						},
					},
				},
			},
			wantUpdated: false,
			wantRule: &cpusetRule{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "update rule with same",
			fields: fields{
				rule: &cpusetRule{
					sharePools: []ext.CPUSharedPool{
						{
							Socket: 0,
							Node:   0,
							CPUSet: "0-7",
						},
						{
							Socket: 1,
							Node:   0,
							CPUSet: "8-15",
						},
					},
				},
			},
			args: args{
				nodeTopo: &topov1alpha1.NodeResourceTopology{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				},
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
			},
			wantUpdated: false,
			wantRule: &cpusetRule{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update rule success",
			fields: fields{
				rule: nil,
			},
			args: args{
				nodeTopo: &topov1alpha1.NodeResourceTopology{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				},
				cpuPolicy: &ext.KubeletCPUManagerPolicy{
					Policy: ext.KubeletCPUManagerPolicyNone,
				},
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
				systemQOSRes: &ext.SystemQOSResource{
					CPUSet: "16-17",
				},
			},
			wantUpdated: true,
			wantRule: &cpusetRule{
				kubeletPolicy: ext.KubeletCPUManagerPolicy{
					Policy: ext.KubeletCPUManagerPolicyNone,
				},
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
				systemQOSCPUSet: "16-17",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &cpusetPlugin{
				rule: tt.fields.rule,
			}
			if tt.args.nodeTopo.Annotations == nil {
				tt.args.nodeTopo.Annotations = map[string]string{}
			}
			if tt.args.cpuPolicy != nil {
				cpuPolicyJson := util.DumpJSON(tt.args.cpuPolicy)
				tt.args.nodeTopo.Annotations[ext.AnnotationKubeletCPUManagerPolicy] = cpuPolicyJson
			}
			if len(tt.args.sharePools) != 0 {
				sharePoolJson := util.DumpJSON(tt.args.sharePools)
				tt.args.nodeTopo.Annotations[ext.AnnotationNodeCPUSharedPools] = sharePoolJson
			}
			if tt.args.systemQOSRes != nil {
				systemQOSJson := util.DumpJSON(tt.args.systemQOSRes)
				tt.args.nodeTopo.Annotations[ext.AnnotationNodeSystemQOSResource] = systemQOSJson
			}
			got, err := p.parseRule(tt.args.nodeTopo)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantUpdated {
				t.Errorf("parseRule() got = %v, wantUpdated %v", got, tt.wantUpdated)
			}
			assert.Equal(t, tt.wantRule, p.rule, "after plugin rule parse")
		})
	}
}

func Test_cpusetPlugin_ruleUpdateCbForPods(t *testing.T) {
	type testPod struct {
		pod       *corev1.Pod
		sandboxID string
	}
	type args struct {
		rule      *cpusetRule
		pods      []*testPod
		podAllocs map[string]ext.ResourceStatus
	}
	type wants struct {
		containersCPUSet map[string]string
		sandboxCPUSet    map[string]string
	}
	tests := []struct {
		name    string
		args    args
		wants   wants
		wantErr bool
	}{
		{
			name: "set container cpuset",
			args: args{
				rule: &cpusetRule{
					sharePools: []ext.CPUSharedPool{
						{
							Socket: 0,
							Node:   0,
							CPUSet: "0-1,5-7",
						},
					},
				},
				pods: []*testPod{
					{
						pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								UID: "pod-with-cpuset-alloc-uid",
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "container-with-cpuset-alloc-name",
									},
								},
							},
							Status: corev1.PodStatus{
								ContainerStatuses: []corev1.ContainerStatus{
									{
										Name:        "container-with-cpuset-alloc-name",
										ContainerID: "containerd://container-with-cpuset-alloc-uid",
									},
								},
							},
						},
						sandboxID: "containerd://pod-with-cpuset-alloc-sandbox-id",
					},
					{
						pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								UID: "pod-cpu-share-uid",
								Labels: map[string]string{
									ext.LabelPodQoS: string(ext.QoSLS),
								},
							},
							Spec: corev1.PodSpec{
								InitContainers: []corev1.Container{
									{
										Name: "init-container-with-cpu-share-name",
									},
								},
								Containers: []corev1.Container{
									{
										Name: "container-with-cpu-share-name",
									},
								},
							},
							Status: corev1.PodStatus{
								InitContainerStatuses: []corev1.ContainerStatus{
									{
										Name:        "init-container-with-cpu-share-name",
										ContainerID: "containerd://init-container-with-cpu-share-uid",
									},
								},
								ContainerStatuses: []corev1.ContainerStatus{
									{
										Name:        "container-with-cpu-share-name",
										ContainerID: "containerd://container-with-cpu-share-uid",
									},
								},
							},
						},
						sandboxID: "containerd://pod-cpu-share-sandbox-id",
					},
					{
						pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								UID: "pod-with-bad-cpuset-alloc-uid",
								Annotations: map[string]string{
									ext.AnnotationResourceStatus: "bad-format",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "container-with-bad-cpuset-alloc-name",
									},
								},
							},
							Status: corev1.PodStatus{
								ContainerStatuses: []corev1.ContainerStatus{
									{
										Name:        "container-with-bad-cpuset-alloc-name",
										ContainerID: "containerd://container-with-bad-cpuset-alloc-uid",
									},
								},
							},
						},
						sandboxID: "containerd://pod-with-bad-cpuset-alloc-sandbox-id",
					},
				},
				podAllocs: map[string]ext.ResourceStatus{
					"pod-with-cpuset-alloc-uid": {
						CPUSet: "2-4",
					},
				},
			},
			wants: wants{
				containersCPUSet: map[string]string{
					"container-with-cpuset-alloc-name":     "2-4",
					"init-container-with-cpu-share-name":   "0-1,5-7",
					"container-with-cpu-share-name":        "0-1,5-7",
					"container-with-bad-cpuset-alloc-name": "",
				},
				sandboxCPUSet: map[string]string{
					"pod-with-cpuset-alloc-uid":     "2-4",
					"pod-cpu-share-uid":             "0-1,5-7",
					"pod-with-bad-cpuset-alloc-uid": "",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHelper := system.NewFileTestUtil(t)

			podUIDMetas := make(map[string]*statesinformer.PodMeta, len(tt.args.pods))
			podUIDCgroupDirs := make(map[string]string, len(tt.args.pods))
			for i := range tt.args.pods {
				podUIDMetas[string(tt.args.pods[i].pod.UID)] = &statesinformer.PodMeta{
					Pod:       tt.args.pods[i].pod,
					CgroupDir: koordletutil.GetPodCgroupParentDir(tt.args.pods[i].pod),
				}
				podUIDCgroupDirs[string(tt.args.pods[i].pod.UID)] = tt.args.pods[i].sandboxID
			}

			// init cgroups cpuset file
			for _, testPod := range tt.args.pods {
				podMeta := podUIDMetas[string(testPod.pod.UID)]
				for _, initContainerStat := range podMeta.Pod.Status.InitContainerStatuses {
					containerPath, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, initContainerStat.ContainerID)
					assert.NoError(t, err, "get init container cgroup path during init container cpuset")
					initCPUSet(containerPath, "", testHelper)
				}
				for _, containerStat := range podMeta.Pod.Status.ContainerStatuses {
					containerPath, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, containerStat.ContainerID)
					assert.NoError(t, err, "get container cgroup path during init container cpuset")
					initCPUSet(containerPath, "", testHelper)
				}
				sandboxPath, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, testPod.sandboxID)
				assert.NoError(t, err, "get sandbox cgroup path during init container cpuset")
				initCPUSet(sandboxPath, "", testHelper)
			}

			// init pod annotations
			for _, testPod := range tt.args.pods {
				podMeta := podUIDMetas[string(testPod.pod.UID)]
				podUID := string(podMeta.Pod.UID)
				podAlloc, exist := tt.args.podAllocs[podUID]
				if !exist {
					continue
				}
				podAllocJson := util.DumpJSON(podAlloc)
				podMeta.Pod.Annotations = map[string]string{
					ext.AnnotationResourceStatus: podAllocJson,
				}
			}

			p := &cpusetPlugin{executor: resourceexecutor.NewResourceUpdateExecutor(), rule: tt.args.rule}
			stop := make(chan struct{})
			defer func() { close(stop) }()
			p.executor.Run(stop)

			podMetas := make([]*statesinformer.PodMeta, 0, len(tt.args.pods))
			for _, podMeta := range podUIDMetas {
				podMetas = append(podMetas, podMeta)
			}
			target := &statesinformer.CallbackTarget{
				Pods: podMetas,
			}

			if err := p.ruleUpdateCb(target); (err != nil) != tt.wantErr {
				t.Errorf("ruleUpdateCb() error = %v, wantErr %v", err, tt.wantErr)
			}

			for _, testPod := range tt.args.pods {
				podMeta := podUIDMetas[string(testPod.pod.UID)]
				for _, initContainerStat := range podMeta.Pod.Status.InitContainerStatuses {
					containerPath, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, initContainerStat.ContainerID)
					assert.NoError(t, err, "get init contaienr cgorup path during check container cpuset")
					gotCPUSet := getCPUSet(containerPath, testHelper)
					assert.Equal(t, tt.wants.containersCPUSet[initContainerStat.Name], gotCPUSet,
						"container cpuset after callback should be equal")
				}

				for _, containerStat := range podMeta.Pod.Status.ContainerStatuses {
					containerPath, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, containerStat.ContainerID)
					assert.NoError(t, err, "get contaienr cgorup path during check container cpuset")
					gotCPUSet := getCPUSet(containerPath, testHelper)
					assert.Equal(t, tt.wants.containersCPUSet[containerStat.Name], gotCPUSet,
						"container cpuset after callback should be equal")
				}

				sandboxPath, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, testPod.sandboxID)
				assert.NoError(t, err, "get sandbox cgorup path during check container cpuset")
				gotCPUSet := getCPUSet(sandboxPath, testHelper)
				assert.Equal(t, tt.wants.sandboxCPUSet[string(podMeta.Pod.UID)], gotCPUSet,
					"sandbox cpuset after callback should be equal")
			}
		})
	}
}

func Test_cpusetRule_getHostAppCpuset(t *testing.T) {
	type fields struct {
		sharePools []ext.CPUSharedPool
	}
	type args struct {
		hostAppReq *protocol.HostAppRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *string
		wantErr bool
	}{
		{
			name: "get nil result with nil request",
			fields: fields{
				sharePools: nil,
			},
			args: args{
				hostAppReq: nil,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "get nil result with bad request qos",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				hostAppReq: &protocol.HostAppRequest{
					Name:         "test-app",
					QOSClass:     ext.QoSLSR,
					CgroupParent: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get cpuset result with ls qos request",
			fields: fields{
				sharePools: []ext.CPUSharedPool{
					{
						Socket: 0,
						Node:   0,
						CPUSet: "0-7",
					},
					{
						Socket: 1,
						Node:   0,
						CPUSet: "8-15",
					},
				},
			},
			args: args{
				hostAppReq: &protocol.HostAppRequest{
					Name:         "test-app",
					QOSClass:     ext.QoSLS,
					CgroupParent: "",
				},
			},
			want:    pointer.String("0-7,8-15"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &cpusetRule{
				sharePools: tt.fields.sharePools,
			}
			got, err := r.getHostAppCpuset(tt.args.hostAppReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("getHostAppCpuset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHostAppCpuset() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cpusetPlugin_ruleUpdateCbForHostApp(t *testing.T) {
	type fields struct {
		rule     *cpusetRule
		executor resourceexecutor.ResourceUpdateExecutor
	}
	type args struct {
		hostApp slov1alpha1.HostApplicationSpec
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantCPUSet string
		wantErr    bool
	}{
		{
			name: "set cpuset for host application",
			fields: fields{
				rule: &cpusetRule{
					sharePools: []ext.CPUSharedPool{
						{
							Socket: 0,
							Node:   0,
							CPUSet: "0-7",
						},
						{
							Socket: 1,
							Node:   0,
							CPUSet: "8-15",
						},
					},
				},
			},
			args: args{
				hostApp: slov1alpha1.HostApplicationSpec{
					Name: "test-app",
					QoS:  ext.QoSLS,
					CgroupPath: &slov1alpha1.CgroupPath{
						ParentDir:    "test-ls",
						RelativePath: "test-app",
					},
				},
			},
			wantCPUSet: "0-7,8-15",
			wantErr:    false,
		},
		{
			name: "set empty cpuset for LSR host application",
			fields: fields{
				rule: &cpusetRule{
					sharePools: []ext.CPUSharedPool{
						{
							Socket: 0,
							Node:   0,
							CPUSet: "0-7",
						},
						{
							Socket: 1,
							Node:   0,
							CPUSet: "8-15",
						},
					},
				},
			},
			args: args{
				hostApp: slov1alpha1.HostApplicationSpec{
					Name: "test-app",
					QoS:  ext.QoSLSR,
					CgroupPath: &slov1alpha1.CgroupPath{
						ParentDir:    "test-ls",
						RelativePath: "test-app",
					},
				},
			},
			wantCPUSet: "",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHelper := system.NewFileTestUtil(t)
			testApp := tt.args.hostApp
			if testApp.CgroupPath == nil ||
				(testApp.CgroupPath.Base != "" && testApp.CgroupPath.Base != slov1alpha1.CgroupBaseTypeRoot) {
				t.Errorf("only cgroup root dir is suupported")
			}

			cgroupDir := filepath.Join(testApp.CgroupPath.ParentDir, testApp.CgroupPath.RelativePath)
			initCPUSet(cgroupDir, "", testHelper)
			p := &cpusetPlugin{
				rule:     tt.fields.rule,
				executor: resourceexecutor.NewResourceUpdateExecutor(),
			}
			stop := make(chan struct{})
			defer func() { close(stop) }()
			p.executor.Run(stop)

			target := &statesinformer.CallbackTarget{
				HostApplications: []slov1alpha1.HostApplicationSpec{tt.args.hostApp},
			}
			if err := p.ruleUpdateCb(target); (err != nil) != tt.wantErr {
				t.Errorf("ruleUpdateCb() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotCPUSet := getCPUSet(cgroupDir, testHelper)
			assert.Equal(t, tt.wantCPUSet, gotCPUSet)
		})
	}
}
