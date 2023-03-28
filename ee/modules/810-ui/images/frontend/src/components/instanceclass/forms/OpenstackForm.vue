<template>
  <GridBlock>
    <CardBlock notice-placement="top">
      <template #title>
        <CardTitle title="Конфигурация" icon="IconOpenStackLogo" />
      </template>
      <template #content>
        <div class="flex flex-wrap items-start -mx-24 -my-6 mb-6">
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
            <Field :name="'flavorName'" v-slot="{ errorMessage }">
              <InputBlock
                title="Тип"
                type="column"
                spec="spec.flavorName"
                class="mb-6"
                :disabled="readonly"
                :error-message="errorMessage"
                required
              >
                <InputText
                  class="p-inputtext-sm w-[450px]"
                  :class="{ 'p-invalid': errorMessage }"
                  v-model="values.flavorName"
                  :disabled="readonly"
                />
              </InputBlock>
            </Field>

            <FieldGroupTitle title="Диск" />
            <div class="flex flex-col gap-y-6">
              <Field :name="'rootDiskSize'" v-slot="{ errorMessage }">
                <InputBlock
                  title="Размер ГБ"
                  spec="spec.rootDiskSize"
                  help="Этот параметр влияет на тип диска<br> <a href='https://deckhouse.ru/documentation/v1/modules/030-cloud-provider-openstack/faq.html#%D0%BA%D0%B0%D0%BA-%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D1%82%D1%8C-rootdisksize-%D0%B8-%D0%BA%D0%BE%D0%B3%D0%B4%D0%B0-%D0%BE%D0%BD-%D0%BF%D1%80%D0%B5%D0%B4%D0%BF%D0%BE%D1%87%D1%82%D0%B8%D1%82%D0%B5%D0%BB%D0%B5%D0%BD' target='_blank' class='text-blue-500'>Как подобрать размер диска</a>"
                  :disabled="readonly"
                  :error-message="errorMessage"
                >
                  <InputNumber class="p-inputtext-sm" :disabled="readonly" v-model="values.rootDiskSize" />
                </InputBlock>
              </Field>
            </div>
          </div>

          <div class="mx-24 my-6">
            <Field :name="'imageName'" v-slot="{ errorMessage }">
              <InputBlock
                title="Образ машины"
                spec="spec.imageName"
                type="column"
                :disabled="readonly"
                :error-message="errorMessage"
              >
                <InputText class="p-inputtext-sm w-[450px]" v-model="values.imageName" :disabled="readonly" />
              </InputBlock>
            </Field>
          </div>
        </div>

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <FieldGroupTitle title="Сеть" />

            <div class="flex flex-col gap-y-6">
              <Field :name="'mainNetwork'" v-slot="{ errorMessage }">
                <InputBlock title="Основная сеть" spec="spec.mainNetwork" type="column" :disabled="readonly" :error-message="errorMessage">
                  <InputText class="p-inputtext-sm w-[450px]" v-model="values.mainNetwork" :disabled="readonly" />
                </InputBlock>
              </Field>
              <FieldArray :name="'additionalNetworks'" v-slot="{ fields, push, remove }" v-model="values.additionalNetworks">
                <InputLabelGroup
                  :model="fields"
                  @push="push({ value: '' })"
                  @remove="remove"
                  title="Дополнительные сети"
                  spec="spec.additionalNetworks"
                  :fields="['network']"
                  :disabled="readonly"
                />
              </FieldArray>
            </div>
          </div>

          <div class="mx-24 my-6">
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
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-12 -my-6">
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

import { z } from "zod";
import { toFormValidator } from "@vee-validate/zod";
import { Field, FieldArray, useForm } from "vee-validate";

import InputText from "primevue/inputtext";
import InputNumber from "primevue/inputnumber";

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
  flavorName: z.string(),
  rootDiskSize: z.number().optional(),
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
  name: props.item.name,
  flavorName: props.item.spec.flavorName,
  rootDiskSize: props.item.spec?.rootDiskSize,
  mainNetwork: props.item.spec?.mainNetwork,
  imageName: props.item.spec?.imageName,
  additionalSecurityGroups: props.item.spec.additionalSecurityGroups?.map((n: string) => ({ value: n })) || [],
  additionalNetworks: props.item.spec.additionalNetworks?.map((n: string) => ({ network: n })) || [],
  additionalTagsAsArray: objectAsArray(props.item.spec?.additionalTags),
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

    newSpec.flavorName = values.flavorName;
    newSpec.rootDiskSize = values.rootDiskSize;
    newSpec.additionalNetworks = values.additionalNetworks.map((i: { network: string }) => i.network);
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
