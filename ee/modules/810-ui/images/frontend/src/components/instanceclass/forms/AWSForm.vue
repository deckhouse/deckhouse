<template>
  <GridBlock>
    <CardBlock notice-placement="top">
      <template #title>
        <CardTitle title="Конфигурация" icon="IconAWSLogo" />
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
            <Field :name="'instanceType'" v-slot="{ errorMessage }">
              <InputBlock
                title="Тип"
                type="column"
                spec="spec.instanceType"
                class="mb-6"
                :disabled="readonly"
                :error-message="errorMessage"
                required
              >
                <InputText
                  class="p-inputtext-sm w-[450px]"
                  :class="{ 'p-invalid': errorMessage }"
                  v-model="values.instanceType"
                  :disabled="readonly"
                />
              </InputBlock>
            </Field>
            <FieldGroupTitle title="Диск" />
            <div class="flex flex-col gap-y-6 mb-6">
              <Field :name="'diskSizeGb'" v-slot="{ errorMessage }">
                <InputBlock title="Размер ГБ" spec="spec.diskSizeGb" :disabled="readonly" :error-message="errorMessage">
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.diskSizeGb" />
                </InputBlock>
              </Field>

              <Field :name="'diskType'" v-slot="{ errorMessage }">
                <InputBlock title="Тип" type="column" spec="spec.diskType" :disabled="readonly" :error-message="errorMessage">
                  <InputText
                    class="p-inputtext-sm w-[450px]"
                    :class="{ 'p-invalid': errorMessage }"
                    v-model="values.diskType"
                    :disabled="readonly"
                  />
                </InputBlock>
              </Field>
            </div>

            <Field :name="'IOPS'" v-slot="{ errorMessage }">
              <InputBlock title="IOPS" spec="spec.IOPS" :disabled="readonly" :error-message="errorMessage">
                <InputNumber class="p-inputtext-sm" :disabled="true" v-model="values.IOPS" />
              </InputBlock>
            </Field>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Прочее" />

            <div class="flex flex-col gap-y-6">
              <Field :name="'ami'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Образ машины"
                  spec="spec.ami"
                  type="column"
                  help="<a href='ya.ru' target='_blank' class='text-blue-500'>Список доступных AMI</a>"
                  :error-message="errorMessage"
                  :disabled="readonly"
                >
                  <InputText class="p-inputtext-sm w-[450px]" :disabled="readonly" v-model="values.ami" />
                </InputBlock>
              </Field>
              <Field :name="'spot'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Использовать Spot"
                  spec="spec.spot"
                  tooltip="Spot-инстансы запускаются с минимальной возможной для успешного запуска ценой за час"
                  :error-message="errorMessage"
                  :disabled="readonly"
                >
                  <InputSwitch :disabled="readonly" v-model="values.spot" />
                </InputBlock>
              </Field>
            </div>
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <FieldArray :name="'additionalSecurityGroups'" v-slot="{ fields, push, remove }" v-model="values.additionalSecurityGroups">
            <InputLabelGroup
              class="mx-12 my-6"
              title="Дополнительные группы безопасности"
              spec="spec.additionalSecurityGroups"
              :fields="['value']"
              :disabled="readonly"
              :model="fields"
              @push="push({ value: '' })"
              @remove="remove"
            />
          </FieldArray>
          <FieldArray :name="'additionalTagsAsArray'" v-slot="{ fields, push, remove }" v-model="values.additionalTagsAsArray">
            <InputLabelGroup
              class="mx-12 my-6"
              title="Дополнительные теги"
              spec="spec.additionalTags"
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
  instanceType: z.string(),
  diskSizeGb: z.number().optional(),
  diskType: z.string().optional(),
  ami: z.string().optional(),
  IOPS: z.number().optional(),
  spot: z.boolean().optional(),
  additionalSecurityGroups: z
    .object({
      value: z.string().optional(),
    })
    .array()
    .optional(),
  additionalTagsAsArray: z
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
  instanceType: props.item.spec.instanceType,
  diskSizeGb: props.item.spec.diskSizeGb,
  diskType: props.item.spec.diskType,
  ami: props.item.spec.ami,
  IOPS: props.item.spec.IOPS,
  spot: props.item.spec.spot,
  additionalSecurityGroups: props.item.spec.additionalSecurityGroups?.map((asg: string) => ({ value: asg })),
  additionalTagsAsArray: objectAsArray(props.item.spec.additionalTags),
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

    newSpec.instanceType = values.instanceType;
    newSpec.diskSizeGb = values.diskSizeGb;
    newSpec.diskType = values.diskType;
    newSpec.ami = values.ami;
    newSpec.IOPS = values.IOPS;
    newSpec.spot = values.spot;
    newSpec.additionalSecurityGroups = values.additionalSecurityGroups?.map((obj: { value: string }) => obj.value);
    newSpec.additionalTags = arrayToObject(values.additionalTagsAsArray);

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
