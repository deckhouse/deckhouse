<template>
  <GridBlock>
    <CardBlock notice-placement="top">
      <template #title>
        <CardTitle title="Конфигурация" icon="IconAzureLogo" />
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
            <Field :name="'machineSize'" v-slot="{ errorMessage }">
              <InputBlock
                title="Тип машины"
                type="column"
                spec="spec.machineSize"
                class="mb-6"
                :disabled="readonly"
                :error-message="errorMessage"
                required
              >
                <InputText
                  class="p-inputtext-sm w-[450px]"
                  :class="{ 'p-invalid': errorMessage }"
                  v-model="values.machineSize"
                  :disabled="readonly"
                />
              </InputBlock>
            </Field>

            <FieldGroupTitle title="Диск" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'diskSizeGb'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Размер ГБ"
                  spec="spec.diskSizeGb"
                >
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

          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Прочее" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'urn'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Образ машины"
                  spec="spec.urn"
                  type="column"
                  help="По умолчанию используется образ группы узлов master<br> <a href='ya.ru' target='_blank' class='text-blue-500'>Список доступных образов</a>"
                >
                  <InputText
                      class="p-inputtext-sm w-[450px]"
                      :class="{ 'p-invalid': errorMessage }"
                      v-model="values.urn"
                      :disabled="readonly"
                    />
                </InputBlock>
              </Field>

              <Field :name="'acceleratedNetworking'" v-slot="{ errorMessage }">
                <InputBlock title="Ускоренная сеть" spec="spec.acceleratedNetworking" special>
                  <InputSwitch :disabled="readonly" v-model="values.acceleratedNetworking" />
                </InputBlock>
              </Field>
            </div>
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <FieldArray :name="'additionalTags'" v-slot="{ fields, push, remove }" v-model="values.additionalTags">
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
  machineSize: z.string(),
  diskSizeGb: z.number().optional(),
  diskType: z.string().optional(),
  urn: z.string().optional(),
  acceleratedNetworking: z.boolean().optional(),
  additionalTags: z
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
  machineSize: props.item.spec.machineSize,
  diskSizeGb: props.item.spec.diskSizeGb,
  diskType: props.item.spec.diskType,
  urn: props.item.spec.urn,
  acceleratedNetworking: props.item.spec.acceleratedNetworking,
  additionalTags: objectAsArray(props.item.spec.additionalTags),
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

    newSpec.machineSize = values.machineSize;
    newSpec.diskSizeGb = values.diskSizeGb;
    newSpec.diskType = values.diskType;
    newSpec.urn = values.urn;
    newSpec.acceleratedNetworking = values.acceleratedNetworking;
    newSpec.additionalTags = arrayToObject(values.additionalTags);

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
