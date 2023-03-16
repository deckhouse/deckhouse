<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <GridBlock mode="form">
    <CardBlock title="Канал обновлений" tooltip="Канал обновлений влияет на множество вещей" class="col-span-2">
      <template #content>
        <SelectButton v-model="releaseChannel" :options="releaseChannelOptions" optionLabel="name" optionValue="value" />
      </template>
    </CardBlock>
    <CardBlock title="Режим обновлений" tooltip="Всегда вручную или же автоамтически? Выбор за вами">
      <template #content>
        <SelectButton v-model="updateMode" :options="updateModeOptions" optionLabel="name" optionValue="value" />
      </template>
    </CardBlock>
    <CardBlock title="Disruptive update" tooltip="Разрешить даже опасные обновления с острым соусом?">
      <template #content>
        <SelectButton v-model="disruptiveUpdateMode" :options="disruptiveUpdateModeOptions" optionLabel="name" optionValue="value" />
      </template>
    </CardBlock>
    <CardBlock title="Окна обновлений" class="col-span-2">
      <template #content>
        <InputRow class="mb-6">
          <MultiSelect v-model="selectedwDays" :options="weekDays" optionLabel="" placeholder="Любой день" />
          <FormLabel value="С" />
          <Dropdown v-model="selectedTimeStart" :options="timeSlots" optionLabel="" />
          <FormLabel value="До" />
          <Dropdown v-model="selectedTimeEnd" :options="timeSlots" optionLabel="" />
          <Button icon="pi pi-times" class="p-button-rounded p-button-danger p-button-outlined" />
        </InputRow>
        <Button label="Добавить" class="p-button-outlined p-button-info w-full" />
      </template>
    </CardBlock>
    <CardBlock title="Уведомить об обновлениях">
      <template #content>
        <div class="flex gap-x-6 items-center mb-6">
          <div>
            <FormLabel value="Оповестить за:" />
            <Dropdown v-model="notifyBefore" :options="notifyBeforeOptions" optionLabel="" />
          </div>
          <div>
            <FormLabel value="Через webhook" />
            <InputText type="text" placeholder="http://example.com" />
          </div>
        </div>
        <FormLabel value="Используя авторизацию" />
        <SelectButton v-model="notifyAuthMode" :options="notifyAuthModeOptions" optionLabel="name" optionValue="value" />

        <div class="flex gap-x-6 items-center mt-6" v-if="notifyAuthMode == 'http'">
          <InputText type="text" placeholder="Логин" />
          <InputText type="text" placeholder="Пароль" />
        </div>
        <div class="mt-6" v-if="notifyAuthMode == 'token'">
          <InputText type="text" placeholder="Token" />
        </div>
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions />
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";

import MultiSelect from "primevue/multiselect";
import SelectButton from "primevue/selectbutton";
import RadioButton from "primevue/radiobutton";
import InputText from 'primevue/inputtext';
import Dropdown from 'primevue/dropdown';
import Button from 'primevue/button';

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";
import ButtonBlock from "@/components/common/button/ButtonBlock.vue"
import InputRow from "@/components/common/form/InputRow.vue";
import FormActions from "@/components/common/form/FormActions.vue";
import FormLabel from "@/components/common/form/FormLabel.vue";

const tabs = [
  {
    id: "1",
    title: "Версии",
    badge: "3+",
  },
  {
    id: "2",
    active: true,
    title: "Настройки обновлений",
  },
];

const weekDays =    ["Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота", "Воскресенье"];
const timeSlots =   ["00:00", "01:00", "02:00", "03:00", "04:00", "05:00", "06:00", "07:00", "08:00", "09:00", "10:00", "11:00", 
                    "12:00", "13:00", "14:00", "15:00", "16:00", "17:00", "18:00", "19:00", "20:00", "21:00", "22:00", "23:00", "00:00"];

const selectedwDays = ref([]);
const selectedTimeStart = ref('00:00');
const selectedTimeEnd = ref('00:00');

const releaseChannel = ref("stable");
const releaseChannelOptions = [{ name: 'Alpha', value: 'alpha'}, { name: 'Beta', value: 'beta'}, { name: 'Stable', value: 'stable'}, { name: 'Early Access', value: 'ea'}];

const updateMode = ref('manual');
const updateModeOptions = [{ name: 'Ручной', value: 'manual'}, { name: 'Авто', value: 'auto'}];

const disruptiveUpdateMode = ref('manual');
const disruptiveUpdateModeOptions = [{ name: 'Ручной', value: 'manual'}, { name: 'Авто', value: 'auto'}];

const notifyBefore = ref('1 час');
const notifyBeforeOptions = ["1 час", "2 часа", "4 часа", "8 часов", "12 часов", "24 часа", "48 часов"];

const notifyAuthMode = ref('none');
const notifyAuthModeOptions = [{ name: 'Нет', value: 'none'}, { name: 'Http-auth', value: 'http'}, { name: 'Token', value: 'token'}];

</script>
