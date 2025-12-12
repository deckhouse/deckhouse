/*
Copyright 2025 Flant JSC

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

package entity

type NodeData struct {
	Name      string
	NodeGroup string
	IsReady   float64
}

type NodeGroupData struct {
	Name      string
	NodeType  string
	HasErrors float64
	Nodes     int32
	Ready     int32
	Max       int32
	Instances int32
	Desired   int32
	Min       int32
	UpToDate  int32
	Standby   int32
}
