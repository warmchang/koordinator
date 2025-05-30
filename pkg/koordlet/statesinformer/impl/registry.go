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

package impl

// NOTE: variables in this file can be overwritten for extension

var DefaultPluginRegistry = map[PluginName]informerPlugin{
	nodeSLOInformerName:      NewNodeSLOInformer(),
	pvcInformerName:          NewPVCInformer(),
	nodeTopoInformerName:     NewNodeTopoInformer(),
	nodeInformerName:         NewNodeInformer(),
	podsInformerName:         NewPodsInformer(),
	podResourcesInformerName: newPodResourcesInformer(),
	nodeMetricInformerName:   NewNodeMetricInformer(),
}
