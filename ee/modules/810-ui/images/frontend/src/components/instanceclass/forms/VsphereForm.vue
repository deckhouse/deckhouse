<template>
  <GridBlock>
    <CardBlock notice-placement="top">
      <template #title>
        <CardTitle title="Конфигурация" icon="IconVmWareLogo" />
      </template>
      <template #content>
        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <Field :name="'name'" v-slot="{ errorMessage }">
              <InputBlock title="Имя" help="Обязательное поле" type="column" class="mb-6" required :error-message="errorMessage">
                <InputText
                  class="p-inputtext-sm w-[500px]"
                  :class="{ 'p-invalid': errorMessage }"
                  v-model="values.name"
                  :disabled="readonly"
                />
              </InputBlock>
            </Field>

             <Field :name="'template'" v-slot="{ errorMessage }">
                <InputBlock title="Идентификатор образа" type="column" spec="spec.template" class="mb-6" :error-message="errorMessage" help="Установлено значение по-умолчанию">
                  <InputText
                    class="p-inputtext-sm w-[500px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.template"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>

            <div class="flex flex-col gap-y-6">
              <Field :name="'numCPUs'" v-slot="{ errorMessage }">
                <InputBlock title="ЦПУ, виртуальных ядер" spec="spec.numCPUs" required>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.numCPUs" />
                </InputBlock>
              </Field>
              <Field :name="'memory'" v-slot="{ errorMessage }">
                <InputBlock title="Память МБ" spec="spec.memory">
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.memory" />
                </InputBlock>
              </Field>
              <Field :name="'resourcePool'" v-slot="{ errorMessage }">
                <InputBlock title="Пул ресурсов" type="column" class="mb-6" :error-message="errorMessage" spec="spec.resourcePool">
                  <InputText
                    class="p-inputtext-sm w-[500px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.resourcePool"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>
              <Field :name="'datastore'" v-slot="{ errorMessage }">
                <InputBlock title="Datastore" type="column" class="mb-6" :error-message="errorMessage" spec="spec.datastore">
                  <InputText
                    class="p-inputtext-sm w-[500px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.datastore"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>
              <Field :name="'rootDiskSize'" v-slot="{ errorMessage }">
                <InputBlock title="Размер корневого диска ГБ" spec="spec.rootDiskSize">
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.rootDiskSize" />
                </InputBlock>
              </Field>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Сеть" />

            <div class="flex flex-col gap-y-6">
              <Field :name="'mainNetwork'" v-slot="{ errorMessage }">
                <InputBlock title="Основная сеть" type="column" spec="spec.mainNetwork" class="mb-6" :disabled="readonly" :error-message="errorMessage">
                  <InputText
                    class="p-inputtext-sm w-[450px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.mainNetwork"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>

              <FieldArray :name="'additionalSubnets'" v-slot="{ fields, push, remove }" v-model="values.additionalSubnets">
                <InputLabelGroup
                  title="Дополнительные подсети"
                  spec="spec.additionalSubnets"
                  :fields="['value']"
                  :disabled="readonly"
                  :model="fields"
                  @push="push({ value: '' })"
                  @remove="remove"
                />
              </FieldArray>
            </div>
          </div>
        </div>

        <CardDivider />

        <CardTitle title="Дополнительные параметры" />

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <div class="flex flex-col gap-y-6">
              <Field :name="'runtimeOptions_cpuLimit'" v-slot="{ errorMessage }">
                <InputBlock title="Верхний лимит потребляемой частоты ЦПУ, МГц" spec="spec.runtimeOptions.cpuLimit" toggle>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.runtimeOptions_cpuLimit" />
                </InputBlock>
              </Field>

              <Field :name="'runtimeOptions_cpuReservation'" v-slot="{ errorMessage }">
                <InputBlock title="Величина зарезервированной потребляемой частоты ЦПУ, МГц" spec="spec.runtimeOptions.cpuReservation" toggle>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.runtimeOptions_cpuReservation" />
                </InputBlock>
              </Field>

              <Field :name="'runtimeOptions_cpuShares'" v-slot="{ errorMessage }">
                <InputBlock title="Относительная величина CPU Shares" spec="spec.runtimeOptions.cpuShares" toggle>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.runtimeOptions_cpuShares" />
                </InputBlock>
              </Field>

              <Field :name="'runtimeOptions_memoryLimit'" v-slot="{ errorMessage }">
                <InputBlock title="Верхний лимит потребляемой памяти МБ" spec="spec.runtimeOptions.memoryLimit" toggle>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.runtimeOptions_memoryLimit" />
                </InputBlock>
              </Field>

              <Field :name="'runtimeOptions_memoryReservations'" v-slot="{ errorMessage }">
                <InputBlock title="Процент зарезервированной памяти в кластере ( % от spec.memory)" spec="spec.runtimeOptions.memoryReservations" help="Допустимые значения 0 < x < 100<br>Значение по умолчанию: 80"  toggle>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.runtimeOptions_memoryReservations" />
                </InputBlock>
              </Field>

              <Field :name="'runtimeOptions_memoryShares'" v-slot="{ errorMessage }">
                <InputBlock title="Относительная величина Memory Shares" spec="spec.runtimeOptions.memoryShares" help="Допустимые значения 0 < x < 100" toggle>
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.runtimeOptions_memoryShares" />
                </InputBlock>
              </Field>
            </div>
          </div>

          <div class="mx-24 my-6">
            <Field :name="'runtimeOptions_nestedHardwareVirtualisation'" v-slot="{ errorMessage }">
              <InputBlock title="Hardware Assisted Virtualization" spec="spec.runtimeOptions.nestedHardwareVirtualisation" special>
                <InputSwitch :disabled="readonly" v-model="values.runtimeOptions_nestedHardwareVirtualisation" />
              </InputBlock>
            </Field>

            <Field :name="'disableTimesync'" v-slot="{ errorMessage }">
              <InputBlock title="Включить синхронизацию времени с ESXi в гостевой ОС" spec="spec.disableTimesync" special>
                <InputSwitch :disabled="readonly" v-model="values.disableTimesync" />
              </InputBlock>
            </Field>
          </div>
        </div>

      </template>
      <template #actions>
        <InstanceClassActions :item="item" />
      </template>
      <template #notice v-if="!!nodeGroupConsumers?.length"> Используется в {{nodeGroupConsumers.length}} группах узлов: <b>{{nodeGroupConsumers.join(', ')}}</b> </template>
    </CardBlock>
  </GridBlock>

  <FormActions :compact="false" v-if="!readonly && meta.dirty" @reset="resetForm" @submit="submitForm" :submit-loading="submitLoading" />
</template>

<script setup lang="ts">
import { computed, ref, type PropType } from "vue";
import { useRouter } from "vue-router";
import { arrayToObject, objectAsArray } from "@/utils";

import type InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";

import { z } from "zod";
import { toFormValidator } from "@vee-validate/zod";
import { Field, FieldArray, useForm } from "vee-validate";

import Dropdown from "primevue/dropdown";
import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";
import InputSwitch from "primevue/inputswitch";
import SelectButton from "primevue/selectbutton";

import InstanceClassActions from "@/components/instanceclass/InstanceClassActions.vue";

import FormActions from "@/components/common/form/FormActions.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import InputBlock from "@/components/common/form/InputBlock.vue";
import InputLabelGroup from "@/components/common/form/InputLabelGroup.vue";
import FieldGroupTitle from "@/components/common/form/FieldGroupTitle.vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardTitle from "@/components/common/card/CardTitle.vue";
import CardDivider from "@/components/common/card/CardDivider.vue";
import type { InstanceClassesTypes } from "@/models/instanceclasses";

const router = useRouter();

const props = defineProps({
  item: {
    type: Object,
    required: true,
  },
  readonly: {
    type: Boolean,
    default: false,
  },
});

const submitLoading = ref(false);

// Validations Schema
const formSchema = z.object({
  name: z.string().min(1),
  numCPUs: z.number(),
  memory: z.number().optional(),
  rootDiskSize: z.number().optional(),
  template: z.string().optional(),
  datastore: z.string().optional(),
  resourcePool: z.string().optional(),
  disableTimesync: z.boolean().optional(),
  mainNetwork: z.string().optional(),

  runtimeOptions_cpuLimit: z.number().optional(),
  runtimeOptions_cpuReservation: z.number().optional(),
  runtimeOptions_cpuShares: z.number().optional(),
  runtimeOptions_memoryLimit: z.number().optional(),
  runtimeOptions_memoryReservations: z.number().optional(),
  runtimeOptions_memoryShares: z.number().optional(),
  runtimeOptions_nestedHardwareVirtualisation: z.boolean().optional(),

  additionalSubnets: z
    .object({
      value: z.string().optional(),
    })
    .array()
    .optional()
});
// Form

const initialValues = computed(() => ({
  name: props.item.metadata.name,
  numCPUs: props.item.spec.numCPUs,
  memory: props.item.spec.memory,
  rootDiskSize: props.item.spec.rootDiskSize,
  template: props.item.spec.template,
  resourcePool: props.item.spec.resourcePool,
  datastore: props.item.spec.datastore,
  disableTimesync: props.item.spec.disableTimesync,
  mainNetwork: props.item.spec.mainNetwork,

  runtimeOptions_cpuLimit: props.item.spec.runtimeOptions?.cpuLimit,
  runtimeOptions_cpuReservation: props.item.spec.runtimeOptions?.cpuReservation,
  runtimeOptions_cpuShares: props.item.spec.runtimeOptions?.cpuShares,
  runtimeOptions_memoryLimit: props.item.spec.runtimeOptions?.memoryLimit,
  runtimeOptions_memoryReservations: props.item.spec.runtimeOptions?.memoryReservations,
  runtimeOptions_memoryShares: props.item.spec.runtimeOptions?.memoryShares,
  runtimeOptions_nestedHardwareVirtualisation: props.item.spec.runtimeOptions?.nestedHardwareVirtualisation,

  additionalSubnets: props.item.spec.additionalSubnets?.map((asg: string) => ({ value: asg })),
}));

const nodeGroupConsumers = computed((): string [] | undefined => {
  return props.item.status?.nodeGroupConsumers;
});

const { handleSubmit, values, meta, resetForm, errors } = useForm({
  validationSchema: toFormValidator(formSchema),
  initialValues,
});

const submitForm = handleSubmit(
  (values) => {
    console.log(values);
    submitLoading.value = true;

    let newSpec = { ...props.item.spec }; // TODO: deepcopy?

    newSpec.numCPUs = values.numCPUs;
    newSpec.memory = values.memory;
    newSpec.rootDiskSize = values.rootDiskSize;
    newSpec.template = newSpec.template;
    newSpec.resourcePool = values.resourcePool;
    newSpec.datastore = values.datastore;
    newSpec.disableTimesync = values.disableTimesync;
    newSpec.mainNetwork = values.mainNetwork;

    newSpec.additionalSubnets = values.additionalSubnets?.map((obj: { value: string }) => obj.value);

    newSpec.runtimeOptions ||= {};
    newSpec.runtimeOptions.cpuLimit = values.runtimeOptions_cpuLimit;
    newSpec.runtimeOptions.cpuReservation = values.runtimeOptions_cpuReservation;
    newSpec.runtimeOptions.cpuShares = values.runtimeOptions_cpuShares;
    newSpec.runtimeOptions.memoryLimit = values.runtimeOptions_memoryLimit;
    newSpec.runtimeOptions.memoryReservations = values.runtimeOptions_memoryReservations;
    newSpec.runtimeOptions.memoryShares = values.runtimeOptions_memoryShares;
    newSpec.runtimeOptions.nestedHardwareVirtualisation = values.runtimeOptions_nestedHardwareVirtualisation;

    props.item.metadata.name = values.name;
    props.item.spec = newSpec;

    props.item.save().then((res: InstanceClassesTypes) => {
      console.log("SAVED", res);
      submitLoading.value = false;

      router.push({ name: "InstanceClassShow", params: { name: props.item.name } });
    });
  },
  (err) => {
    console.error(err);
  }
);
// Functions
</script>
