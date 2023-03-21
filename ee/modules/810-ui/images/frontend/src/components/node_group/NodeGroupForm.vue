<template>
  <GridBlock>
    <CardBlock :title="item.isNew ? '' : 'Статус группы'" notice-placement="top" notice-type="warning" :badges="item.badges">
      <template #content>
        <template v-if="!item.isNew">
          <div class="flex flex-wrap items-start gap-x-12 gap-y-6 mb-6">
            <CardParam title="Тип узлов" :value="item.spec.nodeType" />
            <CardParam title="Приоритет группы" :value="item.priority" />
            <CardParam title="Версия Kubernetes" :value="item.kubernetesVersion" />
          </div>

          <div class="flex flex-wrap items-start gap-x-12 gap-y-6 mb-12">
            <CardParamGroup title="Состояние узлов">
              <CardParamGroupItem title="Всего узлов" :value="String(item.status?.nodes)" />
              <CardParamGroupItem title="Готовые" :value="String(item.status?.ready)" />
              <CardParamGroupItem title="Актуальные" :value="String(item.status?.upToDate)" />
            </CardParamGroup>

            <CardParamGroup title="Параметры автомасштабирования" v-if="item.isAutoscalable" editGoTo="minPerZone">
              <CardParamGroupItem title="Узлов на зону" :value="`${item.status?.min}–${item.status?.max}`" />
              <CardParamGroupItem title="Необходимо" :value="String(item.status?.desired || '—')" />
              <CardParamGroupItem title="Заказано" :value="String(item.status?.instances || '—')" />
              <CardParamGroupItem title="Резерв" :value="`${item.status?.standby}/${item.status?.nodes}`" />
            </CardParamGroup>

            <div class="flex flex-wrap items-start gap-x-12">
              <CardParam title="Зоны" :value="item.zones" v-if="item.isAutoscalable" />
              <CardParam title="Класс машин" :value="item.cloudInstanceKind" v-if="item.isAutoscalable" />
            </div>
          </div>

          <!-- Предварительно решили вынести события в отдельный таб -->

          <!--
        <CardSubtitle title="События (TODO)" />
        <CardTable :fields="table_fields" :data="table_data" />
        -->

          <CardDivider v-if="item.isAutoscalable" />
        </template>
        <template v-if="item.isNew">
          <Field :name="'name'" v-slot="{ errorMessage }">
            <InputBlock title="Имя" help="Обязательное поле" type="column" class="mb-6" required :error-message="errorMessage">
              <InputText
                class="p-inputtext-sm w-[500px]"
                :class="{ 'p-invalid': errorMessage }"
                :disabled="readonly"
                v-model="values.name"
              />
            </InputBlock>
          </Field>
          <CardDivider />
        </template>

        <CardTitle title="Параметры автомасштабирования" v-if="item.isAutoscalable" />
        <div class="flex flex-wrap items-start gap-x-24 gap-y-6 mb-12" v-if="item.isAutoscalable">
          <div class="flex flex-col gap-y-6">
            <Field :name="'cloudInstances.minPerZone'">
              <InputBlock title="Минимум узлов на зону" spec="spec.cloudInstances.minPerZone" ref="minPerZone">
                <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.cloudInstances.minPerZone" :disabled="readonly" />
              </InputBlock>
            </Field>
            <Field :name="'cloudInstances.maxPerZone'">
              <InputBlock title="Максимум узлов на зону" spec="spec.cloudInstances.maxPerZone">
                <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.cloudInstances.maxPerZone" :disabled="readonly" />
              </InputBlock>
            </Field>
            <Field :name="'cloudInstances.instanceClass'">
              <InputBlock title="Класс машин" spec="spec.cloudInstances.classReference" type="column">
                <Dropdown v-model="values.cloudInstances.instanceClass" class="w-full" :options="['TODO']" :disabled="readonly" />
              </InputBlock>
            </Field>
            <Field :name="'cloudInstances.priority'">
              <InputBlock title="Зоны" spec="spec.cloudInstances.zones" type="column">
                <Dropdown v-model="values.cloudInstances.zones" class="w-full" :options="discovery.availableZones" :disabled="readonly" />
              </InputBlock>
            </Field>
          </div>
          <div class="flex flex-col gap-y-6">
            <Field :name="'cloudInstances.priority'">
              <InputBlock title="Приоритет масштабирования" help="Установлено значение по-умолчанию" spec="spec.cloudInstances.priority">
                <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.cloudInstances.priority" :disabled="readonly" />
              </InputBlock>
            </Field>
            <Field :name="'cloudInstances.maxSurgePerZone'">
              <InputBlock
                title="Максимальное количество одновременно обновляемых узлов"
                help="Установлено значение по-умолчанию"
                spec="spec.cloudInsstances.maxSurgePerZone"
              >
                <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.cloudInstances.maxSurgePerZone" :disabled="readonly" />
              </InputBlock>
            </Field>

            <Field :name="'cloudInstances.maxUnavailablePerZone'">
              <InputBlock
                title="MaxUnavailablePerZone"
                help="Максимум недоступных узлов при обновлении"
                spec="spec.cloudInstances.maxUnavailablePerZone"
              >
                <InputNumber
                  inputClass="p-inputtext-sm w-[50px]"
                  v-model="values.cloudInstances.maxUnavailablePerZone"
                  :disabled="readonly"
                />
              </InputBlock>
            </Field>
          </div>
          <div class="flex flex-col gap-y-6">
            <Field :name="'cloudInstances.standBy'">
              <InputBlock title="Число резервных узлов от общего количества узлов" spec="spec.cloudInstances.standBy">
                <div class="p-inputgroup !w-[100px]">
                  <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.cloudInstances.standBy" :disabled="readonly" />
                  <span class="p-inputgroup-addon">из 6</span>
                </div>
              </InputBlock>
            </Field>
            <Field :name="'cloudInstances.standByHolder.overprovisioningRate'">
              <InputBlock
                title="Ресурсы, занимаемые на резервных узлах, %"
                help="Установлено значение по-умолчанию"
                spec="spec.cloudInstances.standByHolder.overprovisioningRate"
              >
                <InputNumber
                  inputClass="p-inputtext-sm w-[50px]"
                  v-model="values.cloudInstances.standByHolder.overprovisioningRate"
                  :disabled="readonly"
                />
              </InputBlock>
            </Field>
            <Field :name="'cloudInstances.quickShutdown'">
              <InputBlock title="Quick Shutdown" help="Снижает время drain'а машины до 5 минут" spec="spec.cloudInstances.quickShutdown">
                <InputSwitch v-model="values.cloudInstances.quickShutdown" :disabled="readonly" />
              </InputBlock>
            </Field>
          </div>
        </div>

        <CardTitle title="Параметры обновлений, влияющие на простой" />
        <div class="flex flex-col gap-y-6">
          <Field :name="'disruptions.approvalMode'">
            <SelectButton
              v-model="values.disruptions.approvalMode"
              :options="updateModeOptions"
              optionLabel="name"
              optionValue="value"
              :unselectable="false"
              :disabled="readonly"
            />
          </Field>

          <UpdateWindows
            v-if="values.disruptions.approvalMode == 'Automatic'"
            v-model="values.disruptions.automatic.windows"
            field-name-path="disruptions.automatic"
            :disabled="readonly"
          />
          <UpdateWindows
            v-if="values.disruptions.approvalMode == 'RollingUpdate'"
            v-model="values.disruptions.rollingUpdate.windows"
            field-name-path="disruptions.rollingUpdate"
            :disabled="readonly"
          />

          <Field :name="'disruptions.automatic.drainBeforeApproval'" v-if="values.disruptions.approvalMode == 'Automatic'">
            <InputBlock title="Выгон подов с узла перед выдачей разрешения" tooltip="Тут будет тултип" type="wide">
              <InputSwitch class="shrink-0" v-model="values.disruptions.automatic.drainBeforeApproval" :disabled="readonly" />
            </InputBlock>
          </Field>
        </div>

        <CardDivider />

        <CardTitle title="Шаблон узла" />
        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <NodeTemplateInputs input-class="mx-12 my-6" :disabled="readonly" v-model="values.nodeTemplate"> </NodeTemplateInputs>
        </div>

        <CardDivider />

        <CardTitle title="Системные параметры узлов" />

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <FieldGroupTitle title="Настройки CRI" tooltip="Тут будет тултип" />
            <div class="flex flex-col gap-y-12 mb-12">
              <Field :name="'cri.type'">
                <SelectButton
                  v-model="values.cri.type"
                  :options="CRIModeOptions"
                  :unselectable="false"
                  optionLabel="name"
                  optionValue="value"
                  :disabled="readonly"
                />
              </Field>
              <Field v-if="values.cri.type == 'Docker'" :name="'cri.docker.maxConcurentDownloads'">
                <InputBlock
                  title="Максимальное количество параллельных потоков загрузки для каждой операции pull"
                  spec="spec.cri.docker.maxConcurentDownloads"
                  :disabled="readonly"
                >
                  <InputNumber
                    :disabled="readonly"
                    inputClass="p-inputtext-sm w-[50px]"
                    v-model="values.cri.docker.maxConcurrentDownloads"
                  />
                </InputBlock>
              </Field>
              <Field v-if="values.cri.type == 'Containerd'" :name="'cri.containerd.maxConcurrentDownloads'">
                <InputBlock
                  title="Максимальное количество параллельных потоков загрузки для каждой операции pull"
                  spec="spec.containerd.maxConcurentDownloads"
                  :disabled="readonly"
                >
                  <InputNumber
                    :disabled="readonly"
                    inputClass="p-inputtext-sm w-[50px]"
                    v-model="values.cri.containerd.maxConcurrentDownloads"
                  />
                </InputBlock>
              </Field>
              <Field v-if="values.cri.type == 'Docker'" :name="'cri.docker.manage'">
                <InputBlock
                  title="Автоматическое управление версией и параметрами Docker"
                  spec="spec.cri.docker.manage"
                  :disabled="readonly"
                >
                  <InputSwitch class="shrink-0" :disabled="readonly" v-model="values.cri.docker.manage" />
                </InputBlock>
              </Field>
            </div>

            <FieldGroupTitle title="Управление ядром" tooltip="Тут будет тултип" />
            <Field :name="'operatingSystem.manageKernel'">
              <InputBlock title="Deckhouse управляет ядром ОС" spec="spec.operatingSystem.manageKernel" :disabled="readonly">
                <InputSwitch :disabled="readonly" v-model="values.operatingSystem.manageKernel" />
              </InputBlock>
            </Field>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Параметры kubelet" tooltip="Тут будет тултип" />
            <div class="flex flex-col gap-y-12 mb-12">
              <Field :name="'kubelet.containerLogMaxFiles'">
                <InputBlock
                  title="Максимальное количество файлов журналов с учетом ротации"
                  spec="spec.kubelet.containerLogMaxFiles"
                  :disabled="readonly"
                >
                  <InputNumber :disabled="readonly" inputClass="p-inputtext-sm w-[50px]" v-model="values.kubelet.containerLogMaxFiles" />
                </InputBlock>
              </Field>
              <Field :name="'kubelet.containerLogMaxSize'">
                <InputBlock
                  title="Максимальный размер файла журнала до ротации, МиБ"
                  spec="spec.kubelet.containerLogMaxSize"
                  :disabled="readonly"
                >
                  <InputText :disabled="readonly" class="p-inputtext-sm w-[50px]" v-model="values.kubelet.containerLogMaxSize" />
                </InputBlock>
              </Field>
              <Field :name="'kubelet.maxPods'">
                <InputBlock
                  title="Максимальное количество pod-ов"
                  help="Установлено значение по-умолчанию"
                  spec="spec.kubelet.maxPods"
                  :disabled="readonly"
                >
                  <InputNumber :disabled="readonly" inputClass="p-inputtext-sm w-[50px]" v-model="values.kubelet.maxPods" />
                </InputBlock>
              </Field>
              <Field :name="'kubelet.rootDir'">
                <InputBlock type="column" title="Путь к каталогу для файлов kubelet" spec="spec.kubelet.rootDir" :disabled="readonly">
                  <InputText :disabled="readonly" class="p-inputtext-sm w-full" v-model="values.kubelet.rootDir" />
                </InputBlock>
              </Field>
            </div>
          </div>
        </div>

        <CardDivider />

        <CardTitle title="Параметры chaos monkey" />
        <div class="flex flex-col gap-y-6 mb-12">
          <Field :name="'chaos.mode'">
            <SelectButton
              v-model="values.chaos.mode"
              :options="chaosOptions"
              :unselectable="false"
              optionLabel="name"
              optionValue="value"
              :disabled="readonly"
            />
          </Field>
          <Field :name="'chaos.period'" v-if="values.chaos.mode && values.chaos.mode != 'Disabled'">
            <InputBlock title="Интервал срабатывания" help="Установлено значение по-умолчанию" spec="chaos.period" :disabled="readonly">
              <!-- TODO: Нужен удобный пикер интервалов в одном инпуте? -->
              <div class="p-inputgroup !w-[100px]">
                <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.chaos.period.hours" :disabled="readonly" />
                <span class="p-inputgroup-addon">час.</span>
              </div>
              <div class="p-inputgroup !w-[100px]">
                <InputNumber inputClass="p-inputtext-sm w-[50px]" v-model="values.chaos.period.mins" :disabled="readonly" />
                <span class="p-inputgroup-addon">мин.</span>
              </div>
            </InputBlock>
          </Field>
        </div>
      </template>
      <template #actions v-if="!item.isNew">
        <NodeGroupActions :item="item" :show-edit="false" />
      </template>
      <template #notice v-if="item.errorMessages.length">
        {{ item.errorMessages.join(";") }}
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions
    :compact="false"
    v-if="(!readonly && meta.dirty) || submitLoading"
    @submit="submitForm"
    @reset="resetForm"
    :submit-loading="submitLoading"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import type { PropType } from "vue";
import { useRouter } from "vue-router";

import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";
import SelectButton from "primevue/selectbutton";
import Dropdown from "primevue/dropdown";
import InputSwitch from "primevue/inputswitch";

import GridBlock from "@/components/common/grid/GridBlock.vue";

import { objectAsArray, arrayToObject, isBlank } from "@/utils";
import { useForm, Field } from "vee-validate";
import { toFormValidator } from "@vee-validate/zod";
import { z, ZodObject } from "zod";

import type NodeGroup from "@/models/NodeGroup";
import Discovery from "@/models/Discovery";

import FormActions from "@/components/common/form/FormActions.vue";
import InputBlock from "@/components/common/form/InputBlock.vue";
import FieldGroupTitle from "@/components/common/form/FieldGroupTitle.vue";
import NodeTemplateInputs from "@/components/common/form/NodeTemplateInputs.vue";
import UpdateWindows from "@/components/common/form/UpdateWindows.vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardTitle from "@/components/common/card/CardTitle.vue";
import CardDivider from "@/components/common/card/CardDivider.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import CardParamGroup from "@/components/common/card/CardParamGroup.vue";
import CardParamGroupItem from "@/components/common/card/CardParamGroupItem.vue";
import CardTable from "@/components/common/card/CardTable.vue";
import NodeGroupActions from "./NodeGroupActions.vue";

import useFormLeaveGuard from "@/composables/useFormLeaveGuard";
import { nodeTemplateSchema, updateWindowSchema } from "@/validations";

const router = useRouter();
const props = defineProps({
  item: {
    type: Object as PropType<NodeGroup>,
    required: true,
  },
  readonly: {
    type: Boolean,
    default: false,
  },
});

const discovery = Discovery.get();
const submitLoading = ref(false);

const updateModeOptions = [
  { name: "Авто", value: "Automatic" },
  { name: "Rolling updates", value: "RollingUpdate" },
  { name: "Ручной", value: "Manual" },
  { name: "По умолчанию", value: undefined },
];

const CRIModeOptions = [
  { name: "Docker", value: "Docker" },
  { name: "Containerd", value: "Containerd" },
  { name: "Not managed", value: "NotManaged" },
  { name: "По умолчанию", value: undefined },
];

const chaosOptions = [
  { name: "Disabled", value: "Disabled" },
  { name: "Drain and delete", value: "DrainAndDelete" },
  { name: "По умолчанию", value: undefined },
];

// const table_fields = ["Created at", "Type", "Reason", "From", "Message"];

// const table_data = [
//   {
//     created_at: "19:00",
//     type: "Normal",
//     reason: "Node not ready",
//     from: "kubelet",
//     message: "Node status is now: node not ready",
//   },
//   {
//     created_at: "19:00",
//     type: "Normal",
//     reason: "Node not ready",
//     from: "kubelet",
//     message: "Node status is now: node not ready",
//   },
//   {
//     created_at: "19:00",
//     type: "Normal",
//     reason: "Node not ready",
//     from: "kubelet",
//     message: "Node status is now: node not ready",
//   },
//   {
//     created_at: "19:00",
//     type: "Normal",
//     reason: "Node not ready",
//     from: "kubelet",
//     message: "Node status is now: node not ready",
//   },
//   {
//     created_at: "19:00",
//     type: "Normal",
//     reason: "Node not ready",
//     from: "kubelet",
//     message: "Node status is now: node not ready",
//   },
// ];

// TODO: DRY
// Validations Schema
const formSchema = ref<ZodObject<any>>();
const formValidator = computed(() => (formSchema.value ? toFormValidator(formSchema.value) : {}));

function updateValidationSchema(newValues: any = null): void {
  let schema = z.object({});

  if (props.item.isNew) schema = schema.merge(z.object({ name: z.string() }));

  if (!newValues) {
    formSchema.value = schema;
    return;
  }

  if (!isBlank(newValues.cloudInsstances)) {
    schema = schema.merge(
      z.object({
        cloudInstances: z.object({
          instanceClass: z.string().optional(),
          zones: z.string().array().optional(),
          priority: z.number().optional(),
          minPerZone: z.number().optional(),
          maxPerZone: z.number().optional(),
          maxUnavailablePerZone: z.number().optional(),
          maxSurgePerZone: z.number().optional(),
          standBy: z.number().optional(),
          standByHolder: z.object({
            overprovisioningRate: z.number().optional(),
          }),
          quickShutdown: z.boolean().optional(),
        }),
      })
    );
  }

  if (!isBlank(newValues.nodeTemplate)) {
    schema = schema.merge(
      z.object({
        nodeTemplate: nodeTemplateSchema,
      })
    );
  }

  if (newValues.disruptions.approvalMode) {
    let disruptionsSchema = z.object({
      approvalMode: z.string(),
    });

    if (newValues.disruptions.approvalMode == "Automatic")
      disruptionsSchema = disruptionsSchema.merge(
        z.object({
          automatic: z.object({
            manage: z.boolean().optional(),
            windows: updateWindowSchema.array(),
          }),
        })
      );
    else if (newValues.disruptions.approvalMode == "RollingUpdate")
      disruptionsSchema = disruptionsSchema.merge(
        z.object({
          rollingUpdate: z.object({
            windows: updateWindowSchema.array(),
          }),
        })
      );

    schema = schema.merge(
      z.object({
        disruptions: disruptionsSchema,
      })
    );
  }

  if (!isBlank(newValues.cri)) {
    schema = schema.merge(
      z.object({
        cri: z.object({
          type: z.string(),
          docker: z.object({
            maxConcurentDownloads: z.number(),
            manage: z.boolean(),
          }),
          containerd: z.object({
            maxConcurentDownloads: z.number(),
          }),
        }),
      })
    );
  }

  if (!isBlank(newValues.operatingSystem)) {
    schema = schema.merge(
      z.object({
        operatingSystem: z.object({
          manageKernel: z.boolean(),
        }),
      })
    );
  }

  if (!isBlank(newValues.kubelet)) {
    schema = schema.merge(
      z.object({
        kubelet: z.object({
          containerLogMaxFiles: z.number(),
          containerLogMaxSize: z.string().min(1),
          maxPods: z.number(),
          rootDir: z.string().optional(),
        }),
      })
    );
  }

  if (!isBlank(newValues.chaos)) {
    let chaosSchema = z.object({
      mode: z.string(),
    });

    if (newValues.chaos.mode == "DrainAndDelete")
      chaosSchema = chaosSchema.merge(
        z.object({
          period: z.object({
            hours: z.number(),
            mins: z.number(),
          }),
        })
      );

    schema = schema.merge(
      z.object({
        chaos: chaosSchema,
      })
    );
  }

  formSchema.value = schema;
}

// Form
const initialValues = computed(() => {
  let vals: any = {
    name: props.item.name || "",
    nodeTemplate: {
      labelsAsArray: objectAsArray(props.item.spec.nodeTemplate?.labels),
      annotationsAsArray: objectAsArray(props.item.spec.nodeTemplate?.annotations),
      taints: props.item.spec.nodeTemplate?.taints || [],
    },
    cloudInstances: {
      instanceClass: props.item.spec.cloudInstances?.classReference?.name,
      zones: props.item.spec.cloudInstances?.zones,
      priority: props.item.spec.cloudInstances?.priority,
      minPerZone: props.item.spec.cloudInstances?.minPerZone,
      maxPerZone: props.item.spec.cloudInstances?.maxPerZone,
      maxUnavailablePerZone: props.item.spec.cloudInstances?.maxUnavailablePerZone,
      maxSurgePerZone: props.item.spec.cloudInstances?.maxSurgePerZone,
      standBy: props.item.spec.cloudInstances?.standby,
      standByHolder: {
        overprovisioningRate: props.item.spec.cloudInstances?.standbyHolder?.overprovisioningRate,
      },
      quickShutdown: props.item.spec.cloudInstances?.quickShutdown,
    },
    disruptions: {
      approvalMode: props.item.spec.disruptions?.approvalMode,
      automatic: props.item.spec.disruptions?.automatic || { windows: [], drainBeforeApproval: null },
      rollingUpdate: props.item.spec.disruptions?.rollingUpdate || { windows: [] },
    },
    cri: {
      type: props.item.spec.cri?.type,
      docker: props.item.spec.cri?.docker || { maxConcurrentDownloads: null, manage: null },
      containerd: props.item.spec.cri?.containerd || { maxConcurrentDownloads: null },
    },
    operatingSystem: {
      manageKernel: props.item.spec.operatingSystem?.manageKernel,
    },
    kubelet: {
      containerLogMaxFiles: props.item.spec.kubelet?.containerLogMaxFiles,
      containerLogMaxSize: props.item.spec.kubelet?.containerLogMaxSize,
      maxPods: props.item.spec.kubelet?.maxPods,
      rootDir: props.item.spec.kubelet?.rootDir,
    },
    chaos: {
      mode: props.item.spec.chaos?.mode,
      period: periodToHoursAndMins(props.item.spec.chaos?.period),
    },
  };

  return vals;
});

const { handleSubmit, values, meta, resetForm, errors, isSubmitting } = useForm({
  validationSchema: formValidator,
  initialValues,
  keepValuesOnUnmount: true, // WARNING: FUCKN IMPORTANT! Without this vee-validate destroys initial-values on unmount by v-if
});

watch(values, updateValidationSchema, { immediate: true });

useFormLeaveGuard({ formMeta: meta, onLeave: resetForm });
// Functions

const submitForm = handleSubmit(
  (values) => {
    console.log(JSON.stringify(values, null, 2));
    console.log(meta.value);

    submitLoading.value = true;

    // eslint-disable-next-line vue/no-mutating-props
    if (props.item.isNew) props.item.metadata = { name: values.name! };

    let newSpec = { ...props.item.spec }; // TODO: deep copy?

    // nodeTemplate
    if (isBlank(values.nodeTemplate)) {
      delete newSpec.nodeTemplate;
    } else {
      newSpec.nodeTemplate = {
        labels: arrayToObject(values.nodeTemplate.labelsAsArray),
        annotations: arrayToObject(values.nodeTemplate.annotationsAsArray),
        taints: values.nodeTemplate.taints,
      };
    }

    // cloudInstances
    if (isBlank(values.cloudInstances)) {
      delete newSpec.cloudInstances;
    } else {
      newSpec.cloudInstances = values.cloudInstances;
    }

    // disruptions
    if (isBlank(values.disruptions)) {
      delete newSpec.disruptions;
    } else {
      newSpec.disruptions = {
        approvalMode: values.disruptions.approvalMode,
      };

      switch (values.disruptions.approvalMode) {
        case "Automatic": {
          newSpec.disruptions.automatic = values.disruptions.automatic;
          break;
        }
        case "RollingUpdate": {
          newSpec.disruptions.rollingUpdate = values.disruptions.rollingUpdate;
          break;
        }
      }
    }

    // operatingSystem
    if (isBlank(values.operatingSystem)) {
      delete newSpec.operatingSystem;
    } else {
      newSpec.cloudInstances = values.operatingSystem;
    }

    // kubelet
    if (isBlank(values.kubelet)) {
      delete newSpec.kubelet;
    } else {
      newSpec.kubelet = values.kubelet;
    }

    // chaos
    if (isBlank(values.chaos)) {
      delete newSpec.chaos;
    } else {
      newSpec.chaos = {
        mode: values.chaos.mode,
      };
      if (values.chaos.period == "DrainAndDelete") newSpec.chaos.period = hoursAndMinsToPeriod(values.chaos.period);
    }

    // eslint-disable-next-line vue/no-mutating-props
    props.item.spec = newSpec;

    props.item.save().then((a: any) => {
      console.log("Save response", a);
      submitLoading.value = false;
      resetForm(); // KOSTYL??
      router.push({ name: "NodeGroupShow", params: { name: props.item.name } });
    });
  },
  (err) => {
    console.error("Validation error", err, errors);
  }
);

type HoursAndMins = { hours?: number; mins?: number };
function periodToHoursAndMins(period: string | undefined): HoursAndMins {
  let res = {} as HoursAndMins;
  if (!period) return res;
  const match = /^((?<hours>\d+)h)?((?<mins>\d+)m)?/.exec(period);
  if (match) {
    res.hours = match.groups?.hours ? parseInt(match.groups.hours) : 0;
    res.mins = match.groups?.mins ? parseInt(match.groups.mins) : 0;

    if (res.mins && res.mins >= 60) {
      res.hours ||= 0;
      res.hours += ~~(res.mins / 60);
      res.mins = res.mins % 60;
    }
  }

  return res;
}

function hoursAndMinsToPeriod(hoursAndMins: HoursAndMins): string {
  if (!hoursAndMins.hours && !hoursAndMins.mins) return "";
  else if (hoursAndMins.hours && !hoursAndMins.mins) return `${hoursAndMins.hours}h`;
  else if (hoursAndMins.mins && !hoursAndMins.hours) return `${hoursAndMins.mins}m`;
  else if (hoursAndMins.hours && hoursAndMins.mins) return `${hoursAndMins.hours * 60 + hoursAndMins.mins}m`;
  return "";
}
</script>
