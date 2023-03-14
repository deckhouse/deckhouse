<template>
  <GridBlock mode="form">
    <CardBlock
      title="Канал обновлений"
      tooltip="Канал обновлений влияет на множество вещей"
      class="col-span-2"
      :content-loading="isLoading"
    >
      <template #content v-if="!isLoading">
        <Field v-model="values.releaseChannel" :name="'releaseChannel'" v-slot="{ handleBlur }">
          <SelectButton
            v-model="values.releaseChannel"
            :options="releaseChannelOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
            @change="handleBlur"
          />
        </Field>
      </template>
    </CardBlock>
    <CardBlock title="Режим обновлений" tooltip="Всегда вручную или же автоамтически? Выбор за вами" :content-loading="isLoading">
      <template #content v-if="!isLoading">
        <Field v-model="values.release.mode" :name="'release.mode'" v-slot="{ handleBlur }">
          <SelectButton
            v-model="values.release.mode"
            :options="releaseModeOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
            @change="handleBlur"
          />
        </Field>
      </template>
    </CardBlock>
    <CardBlock title="Disruptive update" tooltip="Разрешить даже опасные обновления с острым соусом?" :content-loading="isLoading">
      <template #content v-if="!isLoading">
        <Field v-model="values.release.disruptionApprovalMode" :name="'release.disruptionApprovalMode'" v-slot="{ handleBlur }">
          <SelectButton
            v-model="values.release.disruptionApprovalMode"
            :options="disruptionApprovalModeOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
            @change="handleBlur"
          />
        </Field>
      </template>
    </CardBlock>
    <CardBlock title="Окна обновлений" class="col-span-2" :content-loading="isLoading">
      <template #content v-if="!isLoading">
        <FieldArray :name="'release.windows'" v-model="values.release.windows" v-slot="{ fields, push, remove }">
          <InputRow v-for="(window, index) in fields" :key="window.key" class="mb-6">
            <Field :name="`release.windows[${index}].days`" v-slot="{ handleBlur }">
              <MultiSelect
                v-model="values.release.windows[index].days"
                :options="weekDaysOptions"
                optionLabel="name"
                optionValue="value"
                placeholder="Выберите дни"
                @blur="handleBlur"
              />
            </Field>
            <Field :name="`release.windows[${index}].from`" v-slot="{ handleBlur }">
              <FormLabel value="С" />
              <Calendar v-model="values.release.windows[index].from" :showTime="true" :timeOnly="true" @blur="handleBlur" />
            </Field>
            <Field :name="`release.windows[${index}].to`" v-slot="{ handleBlur }">
              <FormLabel value="До" />
              <Calendar v-model="values.release.windows[index].to" :showTime="true" :timeOnly="true" @blur="handleBlur" />
            </Field>
            <Button
              icon="pi pi-times"
              class="p-button-rounded p-button-danger p-button-outlined"
              @click="
                setFieldTouched('release.mode', true);
                remove(index);
              "
            />
          </InputRow>
          <Button
            label="Добавить"
            class="p-button-outlined p-button-info w-full"
            @click="
              push({ days: [], from: '00:00', to: '03:00' });
              setFieldTouched('release.mode', true);
            "
          />
        </FieldArray>
      </template>
    </CardBlock>
    <CardBlock title="Уведомить об обновлениях" v-if="!isLoading && values.release.notification">
      <template #content>
        <div class="flex gap-x-6 items-center mb-6">
          <div>
            <Field
              v-model="values.release.notification.minimalNotificationTime"
              :name="'release.notification.minimalNotificationTime'"
              v-slot="{ handleBlur }"
            >
              <FormLabel value="Оповестить за:" />
              <Dropdown
                v-model="values.release.notification.minimalNotificationTime"
                :options="notifyBeforeOptions"
                optionLabel="name"
                optionValue="value"
                @blur="handleBlur"
              />
            </Field>
          </div>
          <div>
            <Field
              v-model="values.release.notification.webhook"
              :name="'release.notification.webhook'"
              v-slot="{ handleBlur, errorMessage }"
            >
              <FormLabel value="Через webhook" />
              <InputText
                v-model="values.release.notification.webhook"
                type="text"
                :class="{ 'p-invalid': errorMessage }"
                placeholder="http://example.com"
                @blur="handleBlur"
              />
              <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
            </Field>
          </div>
        </div>
        <Field v-model="notificationAuthMode" :name="'notificationAuthMode'" v-slot="{ handleBlur }">
          <FormLabel value="Используя авторизацию" />
          <SelectButton
            v-model="notificationAuthMode"
            :options="notificationAuthModeOptions"
            optionLabel="name"
            optionValue="value"
            :unselectable="false"
            @change="
              notificationAuthModeChange();
              handleBlur();
            "
          />
        </Field>

        <div class="flex gap-x-6 items-center mt-6" v-if="notificationAuthMode == 'basic'">
          <Field v-model="values.release.notification.auth.basic.username" :name="'release.notification.auth.basic.username'">
            <InputText v-model="values.release.notification.auth.basic.username" type="text" placeholder="Логин" />
          </Field>
          <Field v-model="values.release.notification.auth.basic.password" :name="'release.notification.auth.basic.password'">
            <InputText v-model="values.release.notification.auth.basic.password" type="text" placeholder="Пароль" />
          </Field>
        </div>
        <div class="mt-6" v-if="notificationAuthMode == 'token'">
          <Field v-model="values.release.notification.auth.bearerToken" :name="'release.notification.auth.bearerToken'">
            <InputText v-model="values.release.notification.auth.bearerToken" type="text" placeholder="Token" />
          </Field>
        </div>
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions v-if="!isLoading && meta.touched" @submit="submitForm($event)" @reset="resetForm()" />
</template>
<script setup lang="ts">
import { ref } from "vue";

import DeckhouseModuleSettings, { DeckhouseSettings } from "@/models/DeckhouseModuleSettings";
import type { IDeckhouseModuleRelease } from "@/models/DeckhouseModuleSettings";

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
// import dayjs from "dayjs";

// KOSTYL: we need to save full object for correct saving
const deckhouseModuleSettings = ref<DeckhouseModuleSettings>();
const isLoading = ref(true);

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

const notificationAuthMode = ref("none");
const notificationAuthModeOptions = [
  { name: "Нет", value: "none" },
  { name: "Http-auth", value: "basic" },
  { name: "Token", value: "token" },
];

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
    notification: z
      .object({
        minimalNotificationTime: z.string().optional(),
        webhook: z.string().url().optional(),
        auth: z
          .object({
            basic: z
              .object({
                username: z.string(),
                password: z.string(),
              })
              .optional(),
            bearerToken: z.string().optional(),
          })
          .optional(),
      })
      .optional(),
  }),
});
const {
  handleSubmit,
  values,
  meta,
  setValues,
  setFieldTouched,
  resetForm: doFormReset,
} = useForm({
  validationSchema: toFormValidator(settingsSchema),
});

// Functions

const submitForm = handleSubmit((values) => {
  console.log(JSON.stringify(values, null, 2));
  console.log(meta.value);

  for (const key of Object.keys(values)) {
    if (key in deckhouseModuleSettings.value!.spec.settings) {
      deckhouseModuleSettings.value!.spec.settings[key as keyof DeckhouseSettings] = values[key];
    }
  }

  deckhouseModuleSettings.value!.save().then((a) => {
    console.log(a);
    resetForm();
  });
});

function resetForm(): void {
  doFormReset();
  reload();
}

// TODO: settings setter?
function notificationAuthModeChange() {
  switch (notificationAuthMode.value) {
    case "basic": {
      values.release.notification.auth = { basic: { username: "", password: "" } };
      break;
    }
    case "token": {
      values.release.notification.auth = { bearerToken: "" };
      break;
    }
    default: {
      delete values.release.notification.auth;
    }
  }
}

function reload(): void {
  isLoading.value = true;
  DeckhouseModuleSettings.get().then((res: DeckhouseModuleSettings) => {
    res.spec.settings.release ||= {} as IDeckhouseModuleRelease;
    res.spec.settings.release.notification ||= {};

    // deckhouseSettings.value = new DeckhouseSettings(res.spec.settings);
    deckhouseModuleSettings.value = res;
    setValues(res.spec.settings);
    isLoading.value = false;

    // TODO: settings getter?
    if (res.spec.settings.release.notification?.auth && "basic" in res.spec.settings.release.notification.auth)
      notificationAuthMode.value = "basic";
    else if (res.spec.settings.release.notification?.auth && "bearerToken" in res.spec.settings.release.notification.auth)
      notificationAuthMode.value = "token";
    else notificationAuthMode.value = "none";

    // @ts-ignore
    // DeckhouseModuleSettings.subscribe(); // TODO: Alerts if smth change
  });
}

reload();
</script>
