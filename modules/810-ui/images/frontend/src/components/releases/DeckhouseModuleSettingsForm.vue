<template>
  <GridBlock mode="form">
    <CardBlock title="Канал обновлений" tooltip="Канал обновлений влияет на множество вещей" class="col-span-2">
      <template #content>
        <Field :name="'releaseChannel'">
          <SelectButton
            v-model="values.releaseChannel"
            :options="releaseChannelOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
          />
        </Field>
      </template>
    </CardBlock>
    <CardBlock title="Режим обновлений" tooltip="Всегда вручную или же автоамтически? Выбор за вами">
      <template #content>
        <Field :name="'release.mode'">
          <SelectButton
            v-model="values.release.mode"
            :options="releaseModeOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
          />
        </Field>
      </template>
    </CardBlock>
    <CardBlock title="Disruptive update" tooltip="Разрешить даже опасные обновления с острым соусом?">
      <template #content>
        <Field :name="'release.disruptionApprovalMode'">
          <SelectButton
            v-model="values.release.disruptionApprovalMode"
            :options="disruptionApprovalModeOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
          />
        </Field>
      </template>
    </CardBlock>
    <CardBlock title="Окна обновлений" class="col-span-2">
      <template #content>
        <FieldArray :name="'release.windows'" v-model="values.release.windows" v-slot="{ fields, push, remove }">
          <InputRow v-for="(window, index) in fields" :key="window.key" class="mb-6">
            <Field :name="`release.windows[${index}].days`">
              <MultiSelect
                v-model="values.release.windows[index].days"
                :options="weekDaysOptions"
                optionLabel="name"
                optionValue="value"
                placeholder="Выберите дни"
              />
            </Field>
            <Field :name="`release.windows[${index}].from`">
              <FormLabel value="С" />
              <Calendar v-model="values.release.windows[index].from" :showTime="true" :timeOnly="true" />
            </Field>
            <Field :name="`release.windows[${index}].to`">
              <FormLabel value="До" />
              <Calendar v-model="values.release.windows[index].to" :showTime="true" :timeOnly="true" />
            </Field>
            <Button icon="pi pi-times" class="p-button-rounded p-button-danger p-button-outlined" @click="remove(index)" />
          </InputRow>
          <Button label="Добавить" class="p-button-outlined p-button-info w-full" @click="push({ days: [], from: '00:00', to: '03:00' })" />
        </FieldArray>
      </template>
    </CardBlock>
    <CardBlock title="Уведомить об обновлениях" v-if="values.notificationConfig">
      <template #content>
        <div class="flex gap-x-6 items-center mb-6">
          <div>
            <Field :name="'notificationConfig.minimalNotificationTime'">
              <FormLabel value="Оповестить за:" />
              <Dropdown
                v-model="values.notificationConfig.minimalNotificationTime"
                :options="notifyBeforeOptions"
                optionLabel="name"
                optionValue="value"
              />
            </Field>
          </div>
          <div>
            <Field :name="'notificationConfig.webhook'" v-slot="{ errorMessage }">
              <FormLabel value="Через webhook" />
              <InputText
                v-model="values.notificationConfig.webhook"
                type="text"
                :class="{ 'p-invalid': errorMessage }"
                placeholder="http://example.com"
              />
              <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
            </Field>
          </div>
        </div>
        <Field :name="'notificationAuthMode'">
          <FormLabel value="Используя авторизацию" />
          <SelectButton
            v-model="values.notificationAuthMode"
            :options="notificationAuthModeOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
          />
        </Field>

        <div class="flex gap-x-6 items-center mt-6" v-if="values.notificationAuthMode == 'basic'">
          <Field :name="'notificationBasicAuth.username'" v-slot="{ errorMessage }">
            <InputText
              v-model="values.notificationBasicAuth.username"
              type="text"
              placeholder="Логин"
              :class="{ 'p-invalid': errorMessage }"
            />
            <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
          </Field>
          <Field :name="'notificationBasicAuth.password'" v-slot="{ errorMessage }">
            <InputText
              v-model="values.notificationBasicAuth.password"
              type="text"
              placeholder="Пароль"
              :class="{ 'p-invalid': errorMessage }"
            />
            <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
          </Field>
        </div>
        <div class="mt-6" v-if="values.notificationAuthMode == 'token'">
          <Field :name="'notificationAuthToken'" v-slot="{ errorMessage }">
            <InputText v-model="values.notificationAuthToken" type="text" placeholder="Token" :class="{ 'p-invalid': errorMessage }" />
            <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
          </Field>
        </div>
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions v-if="meta.dirty || submitLoading" @submit="submitForm($event)" @reset="resetForm()" :submit-loading="submitLoading" />
</template>
<script setup lang="ts">
import { computed, ref, type PropType } from "vue";

import type DeckhouseModuleSettings from "@/models/DeckhouseModuleSettings";
import type { IDeckhouseModuleReleaseNotification } from "@/models/DeckhouseModuleSettings";

import MultiSelect from "primevue/multiselect";
import SelectButton from "primevue/selectbutton";
// import RadioButton from "primevue/radiobutton";
import InputText from "primevue/inputtext";
import Dropdown from "primevue/dropdown";
import Button from "primevue/button";
import Calendar from "primevue/calendar";
import InlineMessage from "primevue/inlinemessage";

import GridBlock from "@/components/common/grid/GridBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";
import InputRow from "@/components/common/form/InputRow.vue";
import FormActions from "@/components/common/form/FormActions.vue";
import FormLabel from "@/components/common/form/FormLabel.vue";

import { Field, FieldArray, useForm } from "vee-validate";
import { toFormValidator } from "@vee-validate/zod";
import { z } from "zod";
import useFormLeaveGuard from "@/composables/useFormLeaveGuard";
// import dayjs from "dayjs";

const props = defineProps({
  deckhouseModuleSettings: {
    type: Object as PropType<DeckhouseModuleSettings>,
    required: true,
  },
});

const submitLoading = ref(false);

const weekDaysOptions = [
  { name: "Понедельник", value: "Mon" },
  { name: "Вторник", value: "Tue" },
  { name: "Среда", value: "Wed" },
  { name: "Четверг", value: "Thu" },
  { name: "Пятница", value: "Fri" },
  { name: "Суббота", value: "Sat" },
  { name: "Воскресенье", value: "Sun" },
];

const releaseChannelOptions = [
  { name: "Alpha", value: "Alpha" },
  { name: "Beta", value: "Beta" },
  { name: "Stable", value: "Stable" },
  { name: "Early Access", value: "Early Access" },
];

const releaseModeOptions = [
  { name: "Ручной", value: "Manual" },
  { name: "Авто", value: "Auto" },
];

const disruptionApprovalModeOptions = [
  { name: "Ручной", value: "Manual" },
  { name: "Авто", value: "Auto" },
];

const notifyBeforeOptions = [
  { name: "1 час", value: "1h" },
  { name: "2 часа", value: "2h" },
  { name: "4 часа", value: "4h" },
  { name: "8 часов", value: "8h" },
  { name: "12 часов", value: "12h" },
  { name: "24 часа", value: "24h" },
  { name: "48 часов", value: "48h" },
];

const notificationAuthModeOptions = [
  { name: "Нет", value: "none" },
  { name: "Http-auth", value: "basic" },
  { name: "Token", value: "token" },
];
const notificationAuthModes: string[] = notificationAuthModeOptions.map((t: any) => t.value);

const initialValues = computed(() => {
  let { notification, ...releaseConfig } = props.deckhouseModuleSettings.settings.release;
  let notificationAuthMode: (typeof notificationAuthModes)[number];

  if (!notification || Object.keys(notification).length == 0) notification = { webhook: "", minimalNotificationTime: "" };

  const { auth: notificationAuth, ...notificationConfig } = notification;

  if (notificationAuth && "basic" in notificationAuth) notificationAuthMode = "basic";
  else if (notificationAuth && "bearerToken" in notificationAuth) notificationAuthMode = "token";
  else notificationAuthMode = "none";

  return {
    release: releaseConfig,
    releaseChannel: props.deckhouseModuleSettings.settings.releaseChannel,
    notificationAuthMode: notificationAuthMode,
    notificationConfig: notificationConfig,
    notificationBasicAuth: notificationAuth?.basic || { username: "", password: "" },
    notificationAuthToken: notificationAuth?.bearerToken,
  };
});

// Validations
const settingsSchema = z.object({
  releaseChannel: z.enum(releaseChannelOptions.map((rco) => rco.value) as [string, ...string[]]),
  release: z.object({
    mode: z.enum(releaseModeOptions.map((umo) => umo.value) as [string, ...string[]]),
    disruptionApprovalMode: z.enum(disruptionApprovalModeOptions.map((damo) => damo.value) as [string, ...string[]]).optional(),
    windows: z
      .object({
        days: z.enum(weekDaysOptions.map((wdo) => wdo.value) as [string, ...string[]]).array(),
        from: z.string(),
        to: z.string(),
      })
      .array()
      .optional(),
  }),
  notificationAuthMode: z.enum(notificationAuthModes as [string, ...string[]]),
  notificationConfig: z.object({
    minimalNotificationTime: z.string().optional(),
    webhook: z.union([z.string().url().optional(), z.literal("")]),
  }),
  notificationBasicAuth: z
    .object({
      username: z
        .string()
        .optional()
        .refine((val): boolean => (values.notificationAuthMode == "basic" ? !!val : true)),
      password: z
        .string()
        .optional()
        .refine((val): boolean => (values.notificationAuthMode == "basic" ? !!val : true)),
    })
    .optional(),
  notificationAuthToken: z
    .string()
    .optional()
    .refine((val): boolean => (values.notificationAuthMode == "token" ? !!val : true)),
});

const { handleSubmit, values, meta, resetForm } = useForm({
  validationSchema: toFormValidator(settingsSchema),
  initialValues: initialValues,
});

useFormLeaveGuard({ formMeta: meta, onLeave: resetForm });

// Functions
const submitForm = handleSubmit(
  (values) => {
    console.log(JSON.stringify(values, null, 2));
    console.log(meta.value);
    submitLoading.value = true;
    let notification: IDeckhouseModuleReleaseNotification;

    // TODO: no mutating props?
    // eslint-disable-next-line vue/no-mutating-props
    props.deckhouseModuleSettings.spec.settings.releaseChannel = values.releaseChannel;
    // eslint-disable-next-line vue/no-mutating-props
    props.deckhouseModuleSettings.spec.settings.release = values.release;

    if (values.notificationConfig.minimalNotificationTime && values.notificationConfig.webhook) {
      notification = { ...values.notificationConfig };
      switch (values.notificationAuthMode) {
        case "basic": {
          notification.auth = { basic: values.notificationBasicAuth };
          break;
        }
        case "token": {
          notification.auth = { bearerToken: values.notificationAuthToken };
          break;
        }
      }
      // eslint-disable-next-line vue/no-mutating-props
      props.deckhouseModuleSettings.spec.settings.release.notification = notification;
    }

    props.deckhouseModuleSettings.save().then((a) => {
      submitLoading.value = false;
      // TODO: updated?
      resetForm();
    });
  },
  (err) => {
    console.log("Validation errors", err);
  }
);
</script>
