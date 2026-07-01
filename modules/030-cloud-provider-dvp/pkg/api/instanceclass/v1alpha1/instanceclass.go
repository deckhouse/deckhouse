// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package v1alpha1 contains the DVPInstanceClass CRD root type.
//
// +groupName=deckhouse.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=cloudinstanceclasses
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=cloud-provider-dvp"
// +kubebuilder:storageversion
type DVPInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstanceClassSpec `json:"spec"`
}

type InstanceClassSpec struct {
	VirtualMachine InstanceClassVirtualMachine `json:"virtualMachine"`
	RootDisk       InstanceClassRootDisk       `json:"rootDisk"`
	// +optional
	// Parameters for additional virtual machine disks.
	// +deckhouse:ru:description:value="Параметры дополнительных дисков виртуальной машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Каждый элемент массива описывает отдельный дополнительный диск."
	// +deckhouse:ru:description:value="Для каждого диска необходимо задать параметры `size` и `storageClass`."
	AdditionalDisks []InstanceClassDisk `json:"additionalDisks,omitempty"`
	// Specifies settings for the etcd data disk.
	// +deckhouse:ru:description:value="Параметры диска для etcd."
	EtcdDisk InstanceClassDisk `json:"etcdDisk,omitempty"`
}

// Virtual machine settings for the created node.
//
// > The `runPolicy: AlwaysOnUnlessStoppedManually` policy is used for virtual machines of nodes.
// > This allows the virtual machine to be stopped manually (for example, for maintenance) without triggering an automatic restart.
// +deckhouse:ru:description:value="Настройки виртуальной машины для созданного узла."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="> Для виртуальных машин узлов используется политика запуска `runPolicy: AlwaysOnUnlessStoppedManually`."
// +deckhouse:ru:description:value="> Это позволяет вручную останавливать ВМ (например, для обслуживания) без принудительного автозапуска."
type InstanceClassVirtualMachine struct {
	CPU    InstanceClassVirtualMachineCPU    `json:"cpu"`
	Memory InstanceClassVirtualMachineMemory `json:"memory"`
	// The name of the VirtualMachineClass.
	//
	// Intended for centralized configuration of preferred virtual machine parameters. It allows you to specify CPU instruction sets, resource configuration policies for CPU and memory, and define the ratio between these resources.
	// +deckhouse:ru:description:value="Имя VirtualMachineClass."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Ресурс VirtualMachineClass предназначен для централизованной конфигурации предпочтительных параметров виртуальных машин. Он позволяет задавать инструкции CPU, политики конфигурации ресурсов CPU и памяти для виртуальных машин, а также устанавливать соотношения этих ресурсов."
	VirtualMachineClassName string `json:"virtualMachineClassName"`
	// Defines a bootloader for the virtual machine.
	//
	// * `BIOS`: Use BIOS.
	// * `EFI`: Use Unified Extensible Firmware (EFI/UEFI).
	// * `EFIWithSecureBoot`: Use UEFI/EFI with the Secure Boot support.
	// +deckhouse:ru:description:value="Определяет загрузчик для виртуальной машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="* `BIOS` — используется BIOS;"
	// +deckhouse:ru:description:value="* `EFI` — используется Unified Extensible Firmware (EFI/UEFI);"
	// +deckhouse:ru:description:value="* `EFIWithSecureBoot` — используется UEFI/EFI c поддержкой Secure Boot."
	// +kubebuilder:validation:Enum=BIOS;EFI;EFIWithSecureBoot
	// +kubebuilder:default="EFI"
	Bootloader string `json:"bootloader,omitempty"`
	// Virtual machine run policy.
	//
	// * `AlwaysOn`: The virtual machine should always be running.
	// * `AlwaysOff`: The virtual machine should always be stopped.
	// * `Manual`: The virtual machine state is controlled manually.
	// * `AlwaysOnUnlessStoppedManually`: The virtual machine can be stopped manually (for example, for maintenance), but it will automatically start after a host reboot.
	//
	// +deckhouse:ru:description:value="Политика запуска виртуальной машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="* `AlwaysOn` — виртуальная машина всегда должна быть запущена;"
	// +deckhouse:ru:description:value="* `AlwaysOff` — виртуальная машина всегда должна быть остановлена;"
	// +deckhouse:ru:description:value="* `Manual` — состояние виртуальной машины управляется вручную;"
	// +deckhouse:ru:description:value="* `AlwaysOnUnlessStoppedManually` — виртуальную машину можно остановить вручную (например, для обслуживания), но она автоматически запустится после перезагрузки хоста."
	// +kubebuilder:validation:Enum=AlwaysOn;AlwaysOff;Manual;AlwaysOnUnlessStoppedManually
	// +kubebuilder:default="AlwaysOnUnlessStoppedManually"
	// +deckhouse:XDocExample:value="AlwaysOnUnlessStoppedManually"
	RunPolicy string `json:"runPolicy,omitempty"`
	// Live migration policy for the virtual machine.
	//
	// * `Manual`: Migration is controlled manually.
	// * `Never`: Migration is disabled.
	// * `AlwaysSafe`: Always use safe migration (may fail if VM has a high rate of memory changes).
	// * `PreferSafe`: Prefer safe migration, fallback to forced if needed.
	// * `AlwaysForced`: Always use forced migration with VM slowdown.
	// * `PreferForced`: Prefer forced migration (recommended for master nodes due to high memory activity).
	// +deckhouse:ru:description:value="Политика живой миграции виртуальной машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="* `Manual` — миграция управляется вручную;"
	// +deckhouse:ru:description:value="* `Never` — миграция отключена;"
	// +deckhouse:ru:description:value="* `AlwaysSafe` — всегда использовать безопасную миграцию (может не сработать при высокой скорости изменений памяти ВМ);"
	// +deckhouse:ru:description:value="* `PreferSafe` — предпочитать безопасную миграцию, переключаться на forced при необходимости;"
	// +deckhouse:ru:description:value="* `AlwaysForced` — всегда использовать forced-миграцию с замедлением ВМ;"
	// +deckhouse:ru:description:value="* `PreferForced` — предпочитать forced-миграцию (рекомендуется для master-узлов из-за высокой активности памяти)."
	// +kubebuilder:validation:Enum=Manual;Never;AlwaysSafe;PreferSafe;AlwaysForced;PreferForced
	// +kubebuilder:default="PreferForced"
	// +deckhouse:XDocExample:value="PreferForced"
	LiveMigrationPolicy string `json:"liveMigrationPolicy,omitempty"`
	// Additional labels for a virtual machine resource.
	// +deckhouse:ru:description:value="Дополнительные метки для ресурса виртуальной машины."
	// +deckhouse:XDocExample:value="```yaml\ncluster-owner: user\n```"
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`
	// Additional annotations for a virtual machine resource.
	// +deckhouse:ru:description:value="Дополнительные аннотации для ресурса виртуальной машины."
	// +deckhouse:XDocExample:value="```yaml\ncluster-owner: user\n```"
	// +optional
	AdditionalAnnotations map[string]string `json:"additionalAnnotations,omitempty"`
	// Allows a virtual machine to be assigned to specified DVP nodes.
	// [The same](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/) as in the `spec.nodeSelector` parameter for Kubernetes Pods.
	// +deckhouse:ru:description:value="Позволяет назначить виртуальную машину на указанные узлы DVP."
	// +deckhouse:ru:description:value="[Аналогично](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/) параметру `spec.nodeSelector` в Kubernetes Pods."
	// +optional
	NodeSelector InstanceClassVirtualMachineNodeSelector `json:"nodeSelector,omitempty"`
	// [The same](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) as in the `spec.priorityClassName` parameter for Kubernetes Pods.
	// +deckhouse:ru:description:value="[Аналогично](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) параметру `spec.priorityClassName` в Kubernetes Pods."
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// Allows setting tolerations for virtual machines for a DVP node.
	// [The same](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) as in the `spec.tolerations` parameter in Kubernetes Pods.
	// +deckhouse:ru:description:value="Позволяет задать tolerations для виртуальных машин на узле DVP."
	// +deckhouse:ru:description:value="[Аналогично](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) параметру `spec.tolerations` в Kubernetes Pods."
	// +optional
	Tolerations []InstanceClassVirtualMachineToleration `json:"tolerations,omitempty"`
}

// +deckhouse:ru:description:value="Настройки процессора для виртуальной машины."
// CPU settings for the virtual machine.
type InstanceClassVirtualMachineCPU struct {
	// Number of CPU cores for the virtual machine.
	// +deckhouse:ru:description:value="Количество ядер процессора для виртуальной машины."
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Format=int32
	// +deckhouse:XDocExample:value="4"
	Cores int `json:"cores"`
	// Guaranteed share of CPU fraction that will be allocated to the virtual machine.
	// +deckhouse:ru:description:value="Процент гарантированной доли CPU, которая будет выделена виртуальной машине."
	// +kubebuilder:default="100%"
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Pattern=`^100%$|^[1-9][0-9]?%$`
	// +deckhouse:XDocExample:value="100%"
	// +optional
	CoreFraction string `json:"coreFraction,omitempty"`
}

// +deckhouse:ru:description:value="Определяет параметры памяти для виртуальной машины."
// Specifies the memory settings for the virtual machine.
type InstanceClassVirtualMachineMemory struct {
	// Amount of memory resources allowed for the virtual machine.
	//
	// +deckhouse:ru:description:value="Количество ресурсов памяти, разрешенных для виртуальной машины."
	// +kubebuilder:validation:Pattern=`^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`
	// +deckhouse:XDocExample:value="4Gi"
	Size string `json:"size"`
}

// A node selector represents the union of the results of one or more label queries over a set of nodes.
// That is, it represents the OR of the selectors represented by the node selector terms.
// +deckhouse:ru:description:value="Селектор узлов представляет объединение результатов одного или нескольких запросов по меткам к набору узлов."
// +deckhouse:ru:description:value="Иначе говоря, он представляет логическое OR для условий, заданных в `nodeSelectorTerms`."
type InstanceClassVirtualMachineNodeSelector struct {
	// Required. A list of node selector terms. The terms are ORed.
	//
	// A null or empty node selector term matches no objects. The requirements of them are ANDed.
	// The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.
	// +deckhouse:ru:description:value="Список условий выбора узлов. Условия объединяются логическим OR."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Пустое или null-условие выбора узлов не соответствует ни одному объекту. Требования внутри одного условия объединяются логическим AND."
	// +deckhouse:ru:description:value="Тип `TopologySelectorTerm` реализует подмножество `NodeSelectorTerm`."
	NodeSelectorTerms []InstanceClassVirtualMachineNodeSelectorTerm `json:"nodeSelectorTerms"`
}

// A null or empty node selector term matches no objects.
// The requirements of them are ANDed.
// The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.
// +deckhouse:ru:description:value="Пустое или null-условие выбора узлов не соответствует ни одному объекту."
// +deckhouse:ru:description:value="Требования внутри одного условия объединяются логическим AND."
// +deckhouse:ru:description:value="Тип `TopologySelectorTerm` реализует подмножество `NodeSelectorTerm`."
type InstanceClassVirtualMachineNodeSelectorTerm struct {
	// A list of node selector requirements by node's labels.
	//
	// A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.
	// +deckhouse:ru:description:value="Список требований выбора узла по меткам узла."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Требование выбора узла — это селектор, который содержит значения, ключ и оператор, связывающий ключ со значениями."
	// +optional
	MatchExpressions []InstanceClassVirtualMachineNodeSelectorRequirement `json:"matchExpressions,omitempty"`

	// A list of node selector requirements by node's fields.
	//
	// A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.
	// +deckhouse:ru:description:value="Список требований выбора узла по полям узла."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Требование выбора узла — это селектор, который содержит значения, ключ и оператор, связывающий ключ со значениями."
	// +optional
	MatchFields []InstanceClassVirtualMachineNodeSelectorRequirement `json:"matchFields,omitempty"`
}

// A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.
// +deckhouse:ru:description:value="Требование выбора узла — это селектор, который содержит значения, ключ и оператор, связывающий ключ со значениями."
type InstanceClassVirtualMachineNodeSelectorRequirement struct {
	// The label key that the selector applies to.
	// +deckhouse:ru:description:value="Ключ, к которому применяется селектор."
	Key string `json:"key"`

	// Represents a key's relationship to a set of values.
	// Valid operators are In, NotIn, Exists, DoesNotExist, Gt, and Lt.
	// +deckhouse:ru:description:value="Определяет отношение ключа к набору значений."
	// +deckhouse:ru:description:value="Допустимые операторы: `In`, `NotIn`, `Exists`, `DoesNotExist`, `Gt` и `Lt`."
	// +kubebuilder:validation:Enum=In;NotIn;Exists;DoesNotExist;Gt;Lt
	Operator string `json:"operator"`

	// An array of string values.
	// If the operator is In or NotIn, the values array must be non-empty.
	// If the operator is Exists or DoesNotExist, the values array must be empty.
	// If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer.
	// This array is replaced during a strategic merge patch.
	// +deckhouse:ru:description:value="Массив строковых значений."
	// +deckhouse:ru:description:value="Если оператор — `In` или `NotIn`, массив `values` должен быть непустым."
	// +deckhouse:ru:description:value="Если оператор — `Exists` или `DoesNotExist`, массив `values` должен быть пустым."
	// +deckhouse:ru:description:value="Если оператор — `Gt` или `Lt`, массив `values` должен содержать один элемент, который будет интерпретирован как целое число."
	// +deckhouse:ru:description:value="Этот массив заменяется при применении `strategic merge patch`."
	// +optional
	Values []string `json:"values,omitempty"`
}

// The virtual machine this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator.
// +deckhouse:ru:description:value="Виртуальная машина, для которой задан этот toleration, допускает любой taint, соответствующий тройке `<key,value,effect>` с учетом заданного оператора сопоставления."
type InstanceClassVirtualMachineToleration struct {
	// Key is the taint key that the toleration applies to.
	// Empty means match all taint keys.
	// If the key is empty, operator must be Exists; this combination means to match all values and all keys.
	// +deckhouse:ru:description:value="Ключ taint, к которому применяется toleration."
	// +deckhouse:ru:description:value="Пустое значение означает соответствие всем ключам taint."
	// +deckhouse:ru:description:value="Если ключ пустой, оператор должен быть `Exists`; такая комбинация означает соответствие всем значениям и всем ключам."
	// +optional
	Key string `json:"key,omitempty"`

	// Operator represents a key's relationship to the value.
	// Valid operators are Exists, Equal, Lt, and Gt. Defaults to Equal.
	// Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.
	// Lt and Gt perform numeric comparisons (requires feature gate TaintTolerationComparisonOperators).
	// +deckhouse:ru:description:value="Оператор определяет отношение ключа к значению."
	// +deckhouse:ru:description:value="Допустимые операторы: `Exists`, `Equal`, `Lt` и `Gt`. По умолчанию используется `Equal`."
	// +deckhouse:ru:description:value="Оператор `Exists` эквивалентен wildcard для значения, поэтому виртуальная машина может допускать все taint определенной категории."
	// +deckhouse:ru:description:value="Операторы `Lt` и `Gt` выполняют числовые сравнения и требуют включенного feature gate `TaintTolerationComparisonOperators`."
	// +optional
	Operator string `json:"operator,omitempty"`

	// Value is the taint value the toleration matches to.
	// If the operator is Exists, the value should be empty, otherwise just a regular string.
	// +deckhouse:ru:description:value="Значение taint, с которым сопоставляется toleration."
	// +deckhouse:ru:description:value="Если оператор — `Exists`, значение должно быть пустым. В остальных случаях указывается обычная строка."
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates the taint effect to match.
	// Empty means match all taint effects.
	// When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.
	// +deckhouse:ru:description:value="Указывает эффект taint, с которым выполняется сопоставление."
	// +deckhouse:ru:description:value="Пустое значение означает соответствие всем эффектам taint."
	// +deckhouse:ru:description:value="Если значение указано, допустимые значения: `NoSchedule`, `PreferNoSchedule` и `NoExecute`."
	// +optional
	Effect string `json:"effect,omitempty"`

	// TolerationSeconds represents the period of time the toleration, which must be of effect NoExecute,
	// tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict).
	// Zero and negative values will be treated as 0 (evict immediately) by the system.
	// +deckhouse:ru:description:value="Период времени, в течение которого toleration допускает taint. Поле применяется только для taint с эффектом `NoExecute`; для остальных эффектов оно игнорируется."
	// +deckhouse:ru:description:value="По умолчанию значение не задано, что означает постоянное допущение taint без вытеснения."
	// +deckhouse:ru:description:value="Нулевые и отрицательные значения будут обработаны системой как `0`, то есть приведут к немедленному вытеснению."
	// +optional
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// +deckhouse:ru:description:value="Параметры образа, который будет использоваться для создания корневого диска виртуальной машины."
// Image parameters that will be used to create the virtual machine's root disk.
type InstanceClassImage struct {
	// The kind of the image source.
	// +deckhouse:ru:description:value="Тип источника изображения."
	// +kubebuilder:validation:Enum=ClusterVirtualImage;VirtualImage;VirtualDisk
	Kind string `json:"kind"`
	// The name of the image that will be used to create the root disk.
	//
	// > The installation requires Linux OS images with cloud-init pre-installed.
	// +deckhouse:ru:description:value="Имя образа, который будет использоваться для создания корневого диска."
	// +deckhouse:ru:description:value="> Для установки требуются образы ОС Linux с предустановленным cloud-init."
	Name string `json:"name"`
}

// +deckhouse:ru:description:value="Указывает настройки для корневого диска виртуальной машины."
// Specifies settings for the root disk of the virtual machine.
type InstanceClassRootDisk struct {
	// Root disk size.
	// +deckhouse:ru:description:value="Размер корневого диска."
	// +kubebuilder:validation:Pattern=`^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`
	// +deckhouse:XDocExample:value="10Gi"
	Size string `json:"size"`
	// The name of the existing StorageClass will be used to create the virtual machine's root disk.
	//
	// If the value is not specified, the StorageClass will be used according to the [global storageClass parameter](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass) setting.
	// +deckhouse:ru:description:value="Имя существующего StorageClass будет использоваться для создания корневого диска виртуальной машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Если значение не указано, то будет использоваться StorageClass, согласно настройке [глобального параметра storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass)."
	// +optional
	StorageClass string             `json:"storageClass,omitempty"`
	Image        InstanceClassImage `json:"image"`
}

type InstanceClassDisk struct {
	// Size of the disk.
	// +deckhouse:ru:description:value="Размер дополнительного диска."
	// +kubebuilder:validation:Pattern=`^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`
	// +deckhouse:XDocExample:value="10Gi"
	Size string `json:"size"`
	// Name of the existing StorageClass that will be used to create the disk.
	// +deckhouse:ru:description:value="Имя существующего StorageClass, который будет использоваться для создания дополнительного диска."
	StorageClass string `json:"storageClass"`
}