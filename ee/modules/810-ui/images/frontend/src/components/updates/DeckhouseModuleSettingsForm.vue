<template>
  <GridBlock  mode="form">
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
        <Field v-model="values.update.mode" :name="'update.mode'" v-slot="{ handleBlur }">
          <SelectButton
            v-model="values.update.mode"
            :options="updateModeOptions"
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
        <Field v-model="values.update.disruptionApprovalMode" :name="'update.disruptionApprovalMode'" v-slot="{ handleBlur }">
          <SelectButton
            v-model="values.update.disruptionApprovalMode"
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
        <FieldArray :name="'update.windows'" v-model="values.update.windows" v-slot="{ fields, push, remove }">
          <InputRow v-for="(window, index) in fields" :key="window.key" class="mb-6">
            <Field :name="`update.windows[${index}].days`" v-slot="{ handleBlur }">
              <MultiSelect
                v-model="values.update.windows[index].days"
                :options="weekDaysOptions"
                optionLabel="name"
                optionValue="value"
                placeholder="Выберите дни"
                @blur="handleBlur"
              />
            </Field>
            <Field :name="`update.windows[${index}].from`" v-slot="{ handleBlur }">
              <FormLabel value="С" />
              <Calendar v-model="values.update.windows[index].from" :showTime="true" :timeOnly="true" @blur="handleBlur" />
            </Field>
            <Field :name="`update.windows[${index}].to`" v-slot="{ handleBlur }">
              <FormLabel value="До" />
              <Calendar v-model="values.update.windows[index].to" :showTime="true" :timeOnly="true" @blur="handleBlur" />
            </Field>
            <Button
              icon="pi pi-times"
              class="p-button-rounded p-button-danger p-button-outlined"
              @click="
                setFieldTouched('update.mode', true);
                remove(index);
              "
            />
          </InputRow>
          <Button
            label="Добавить"
            class="p-button-outlined p-button-info w-full"
            @click="
              push({ days: [], from: '00:00', to: '03:00' });
              setFieldTouched('update.mode', true);
            "
          />
        </FieldArray>
      </template>
    </CardBlock>
    <CardBlock title="Уведомить об обновлениях" v-if="!isLoading && values.update.notification">
      <template #content>
        <div class="flex gap-x-6 items-center mb-6">
          <div>
            <Field
              v-model="values.update.notification.minimalNotificationTime"
              :name="'update.notification.minimalNotificationTime'"
              v-slot="{ handleBlur }"
            >
              <FormLabel value="Оповестить за:" />
              <Dropdown
                v-model="values.update.notification.minimalNotificationTime"
                :options="notifyBeforeOptions"
                optionLabel="name"
                optionValue="value"
                @blur="handleBlur"
              />
            </Field>
          </div>
          <div>
            <Field v-model="values.update.notification.webhook" :name="'update.notification.webhook'" v-slot="{ handleBlur, errorMessage }">
              <FormLabel value="Через webhook" />
              <InputText
                v-model="values.update.notification.webhook"
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
          <Field v-model="values.update.notification.auth.basic.username" :name="'update.notification.auth.basic.username'">
            <InputText v-model="values.update.notification.auth.basic.username" type="text" placeholder="Логин" />
          </Field>
          <Field v-model="values.update.notification.auth.basic.password" :name="'update.notification.auth.basic.password'">
            <InputText v-model="values.update.notification.auth.basic.password" type="text" placeholder="Пароль" />
          </Field>
        </div>
        <div class="mt-6" v-if="notificationAuthMode == 'token'">
          <Field v-model="values.update.notification.auth.bearerToken" :name="'update.notification.auth.bearerToken'">
            <InputText v-model="values.update.notification.auth.bearerToken" type="text" placeholder="Token" />
          </Field>
        </div>
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions v-if="!isLoading && meta.touched" @submit="submitForm($event)" @reset="resetForm()" />
</template>
<script setup lang="ts">
import { ref } from "vue";

import DeckhouseModuleSettings from "@/models/DeckhouseModuleSettings";
import type { IDeckhouseModuleUpdate } from "@/models/DeckhouseModuleSettings";

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

const updateModeOptions = [
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
  update: z.object({
    mode: z.enum(updateModeOptions.map((umo) => umo.value) as [string, ...string[]]),
    disruptionApprovalMode: z.enum(disruptionApprovalModeOptions.map((damo) => damo.value) as [string, ...string[]]),
    windows: z
      .object({
        days: z.enum(weekDaysOptions.map((wdo) => wdo.value) as [string, ...string[]]).array(),
        from: z.string(),
        to: z.string(),
      })
      .array(),
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

  deckhouseModuleSettings.value!.spec.settings = Object.assign(deckhouseModuleSettings.value!.spec.settings, values);
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
      values.update.notification.auth = { basic: { username: "", password: "" } };
      break;
    }
    case "token": {
      values.update.notification.auth = { bearerToken: "" };
      break;
    }
    default: {
      delete values.update.notification.auth;
    }
  }
}

function reload(): void {
  isLoading.value = true;
  DeckhouseModuleSettings.get().then((res: DeckhouseModuleSettings) => {
    res.spec.settings.update ||= {} as IDeckhouseModuleUpdate;
    res.spec.settings.update.notification ||= {};

    // deckhouseSettings.value = new DeckhouseSettings(res.spec.settings);
    deckhouseModuleSettings.value = res;
    setValues(res.spec.settings);
    isLoading.value = false;

    // TODO: settings getter?
    if (res.spec.settings.update.notification?.auth && "basic" in res.spec.settings.update.notification.auth)
      notificationAuthMode.value = "basic";
    else if (res.spec.settings.update.notification?.auth && "bearerToken" in res.spec.settings.update.notification.auth)
      notificationAuthMode.value = "token";
    else notificationAuthMode.value = "none";

    // @ts-ignore
    // DeckhouseModuleSettings.subscribe(); // TODO: Alerts if smth change
  });
}

reload();
</script>
