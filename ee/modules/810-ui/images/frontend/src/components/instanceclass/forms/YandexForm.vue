<template>
  <GridBlock>
    <CardBlock notice-placement="top">
      <template #title>
        <CardTitle title="Конфигурация" icon="IconYandexCloudLogo" />
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

            <FieldGroupTitle title="Ресурсы" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'cores'" v-slot="{ errorMessage }">
                <InputBlock
                  title="ЦПУ, виртуальных ядер"
                  spec="spec.cores"
                  required
                >
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.cores" />
                </InputBlock>
              </Field>
              <Field :name="'platformID'" v-slot="{ errorMessage }">
                <InputBlock title="Платформа ЦПУ" type="column" spec="spec.platformID" class="mb-6" :disabled="readonly" :error-message="errorMessage" help="Установлено значение по умолчанию<br> <a href='ya.ru' target='_blank' class='text-blue-500'>Список существующих платформ</a>">
                  <InputText
                    class="p-inputtext-sm w-[450px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.platformID"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>
              <Field :name="'memory'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Память МБ"
                  spec="spec.memory"
                >
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.memory" />
                </InputBlock>
              </Field>
              <Field :name="'coreFraction'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Базовый уровень производительности ядер"
                  spec="spec.coreFraction"
                >
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.coreFraction" />
                </InputBlock>
              </Field>
              <Field :name="'gpus'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Количество графических адаптеров"
                  spec="spec.gpus"
                >
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.gpus" />
                </InputBlock>
              </Field>
            </div>

            <FieldGroupTitle title="Диск" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'diskSizeGB'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Размер ГБ"
                  spec="spec.diskSizeGB"
                >
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.diskSizeGB" />
                </InputBlock>
              </Field>
              <Field :name="'diskType'" v-slot="{ errorMessage }">
                <InputBlock title="Тип" type="column" spec="spec.diskType" class="mb-6" :disabled="readonly" :error-message="errorMessage">
                  <InputText
                    class="p-inputtext-sm w-[450px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.diskType"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Сеть" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'mainSubnet'" v-slot="{ errorMessage }">
                <InputBlock title="Основная подсеть" type="column" spec="spec.mainSubnet" class="mb-6" :disabled="readonly" :error-message="errorMessage">
                  <InputText
                    class="p-inputtext-sm w-[450px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.mainSubnet"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>

              <Field :name="'networkType'" v-slot="{ errorMessage }">
                <InputBlock title="Тип сети" type="column" spec="spec.networkType" class="mb-6" :disabled="readonly" :error-message="errorMessage">
                  <InputText
                    class="p-inputtext-sm w-[450px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.networkType"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>

              <FieldArray :name="'additionalSubnets'" v-slot="{ fields, push, remove }" v-model="values.additionalSubnets">
                <InputLabelGroup
                  title="Дополнительные группы безопасности"
                  spec="spec.additionalSubnets"
                  :fields="['value']"
                  :disabled="readonly"
                  :model="fields"
                  @push="push({ value: '' })"
                  @remove="remove"
                />
              </FieldArray>

              <Field :name="'assignPublicIPAddress'" v-slot="{ errorMessage }">
                <InputBlock title="Публичный IP" spec="spec.assignPublicIPAddress" special>
                  <InputSwitch :disabled="readonly" v-model="values.assignPublicIPAddress" />
                </InputBlock>
              </Field>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Прочее" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'ImageID'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Идентификатор образа"
                  spec="spec.ImageID"
                  type="column"
                  help="По умолчанию используется образ группы узлов master"
                >
                  <InputText
                      class="p-inputtext-sm w-[450px]"
                      :class="{ 'p-invalid': errorMessage }"
                      v-model="values.ImageID"
                      :disabled="readonly"
                    />
                </InputBlock>
              </Field>

              <Field :name="'preemptible'" v-slot="{ errorMessage }">
                <InputBlock title="Прерываемые ВМ" spec="spec.preemptible" special>
                  <InputSwitch :disabled="readonly" v-model="values.preemptible" />
                </InputBlock>
              </Field>
            </div>
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <FieldArray :name="'additionalLabels'" v-slot="{ fields, push, remove }" v-model="values.additionalLabels">
            <InputLabelGroup
              class="mx-12 my-6"
              title="Дополнительные лейблы"
              spec="spec.additionalLabels"
              :disabled="readonly"
              :fields="['key', 'value']"
              :model="fields"
              @push="push({ key: '', value: '' })"
              @remove="remove"
            />
          </FieldArray>
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
  cores: z.number(),
  platformID: z.string().optional(),
  memory: z.number().optional(),
  coreFraction: z.number().optional(),
  gpus: z.number().optional(),
  diskSizeGB: z.number().optional(),
  diskType: z.string().optional(),
  mainSubnet: z.string().optional(),
  networkType: z.string().optional(),
  assignPublicIPAddress: z.boolean().optional(),
  ImageID: z.string().optional(),
  preemptible: z.boolean().optional(),

  additionalSubnets: z
    .object({
      value: z.string().optional(),
    })
    .array()
    .optional(),
  additionalLabels: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
    })
    .array()
    .optional(),
});
// Form

const initialValues = computed(() => ({
  name: props.item.metadata.name,
  cores: props.item.spec.cores,
  platformID: props.item.spec.platformID,
  memory: props.item.spec.memory,
  coreFraction: props.item.spec.coreFraction,
  gpus: props.item.spec.gpus,
  diskSizeGB: props.item.spec.diskSizeGB,
  diskType: props.item.spec.diskType,
  mainSubnet: props.item.spec.mainSubnet,
  networkType: props.item.spec.networkType,
  assignPublicIPAddress: props.item.spec.assignPublicIPAddress,
  ImageID: props.item.spec.ImageID,
  preemptible: props.item.spec.preemptible,
  additionalSubnets: props.item.spec.additionalSubnets?.map((asg: string) => ({ value: asg })),

  additionalLabels: objectAsArray(props.item.spec.additionalLabels),
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

    newSpec.cores = values.cores;
    newSpec.platformID = values.platformID;
    newSpec.memory = values.memory;
    newSpec.coreFraction = values.coreFraction;
    newSpec.gpus = values.gpus;
    newSpec.diskSizeGB = values.diskSizeGB;
    newSpec.diskType = values.diskType;
    newSpec.mainSubnet = values.mainSubnet;
    newSpec.networkType = values.networkType;
    newSpec.assignPublicIPAddress = values.assignPublicIPAddress;
    newSpec.ImageID = values.ImageID;
    newSpec.preemptible = values.preemptible;

    newSpec.additionalSubnets = values.additionalSubnets?.map((obj: { value: string }) => obj.value);
    newSpec.additionalLabels = arrayToObject(values.additionalLabels);

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
