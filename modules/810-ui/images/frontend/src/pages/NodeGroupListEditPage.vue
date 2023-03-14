<template>
  <PageTitle>The-biggest-node-group-name-imaginable-is-63-symbols-max-woohoo</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <GridBlock>
    <CardBlock title="Состояние группы" notice-placement="top" notice-type="warning" :badges="[ { id: 1, title: 'Сломалося', type: 'warning'} ]">
      <template #content>
        <div class="flex flex-wrap items-start gap-x-12 gap-y-6 mb-6">
        <CardParamGroup title="Состояние узлов">
          <CardParamGroupItem title="Всего узлов" value="6" />
          <CardParamGroupItem title="Готовые" value="6" state="danger" />
          <CardParamGroupItem title="Актуальные" value="5" />
        </CardParamGroup>

        <CardParamGroup title="Параметры автомасштабирования">
          <CardParamGroupItem title="Узлов на зону" value="3–10" :edit="true" />
          <CardParamGroupItem title="Необходимо" value="7" />
          <CardParamGroupItem title="Заказано" value="6" />
          <CardParamGroupItem title="Резерв" value="2/6" />
        </CardParamGroup>

        <CardParam title="Зоны">
          <div class="field-checkbox mb-3">
            <Checkbox inputId="zone1" name="zone" value="Chicago" v-model="zones" />
            <label for="zone1" class="text-sm font-medium text-gray-800 ml-1">eu-west-1a</label>
          </div>

          <div class="field-checkbox mb-3">
              <Checkbox inputId="zone2" name="zone" value="eu-west-1b" v-model="zones" />
              <label for="zone2" class="text-sm font-medium text-gray-800 ml-1">eu-west-1b</label>
          </div>

          <div class="field-checkbox mb-3">
              <Checkbox inputId="zone3" name="zone" value="eu-west-1c" v-model="zones" />
              <label for="zone3" class="text-sm font-medium text-gray-800 ml-1">eu-west-1c</label>
          </div>

          <div class="field-checkbox mb-3">
              <Checkbox inputId="zone4" name="zone" value="eu-west-1d" v-model="zones" />
              <label for="zone4" class="text-sm font-medium text-gray-800 ml-1">eu-west-1d</label>
          </div>
        </CardParam>
        <CardParam title="Класс машин" value="Yandex.Cloud" />
      </div>

      <CardDivider />

      <CardTitle title="События" />
      <CardTable :fields="table_fields" :data="table_data" />

      <CardDivider />

      <CardTitle title="Шаблон узла" />
      <div class="flex flex-wrap items-start -mx-12 -my-6">
        <InputLabelGroup class="mx-12 my-6" title="Аннотации" :fields="['Key', 'Value']" />
        <InputLabelGroup class="mx-12 my-6" title="Лейблы" :fields="['Key', 'Value']" />
        <InputLabelGroup class="mx-12 my-6" title="Тейнты" :fields="['Effect', 'Key', 'Value']" />
      </div>

      <CardDivider />

      <div class="flex flex-wrap items-start -mx-24 -my-6">
        <div class="mx-24 my-6">
          <CardTitle title="Параметры автомасштабирования" />
          <div class="flex flex-col gap-y-6">
            <InputBlock title="Приоритет масштабирования" help="Установлено значение по-умолчанию">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="MaxUnavailablePerZone" help="Максимум недоступных узлов при обновлении">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Максимальное количество одновременно обновляемых узлов" help="Установлено значение по-умолчанию">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Stand by" help="Установлено значение по-умолчанию">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Число резервных узлов от общего количества узлов">
                <div class="p-inputgroup !w-[150px]">
                  <InputText class="p-inputtext-sm w-[50px]" />
                  <span class="p-inputgroup-addon">из 6</span>
                </div>
            </InputBlock>
            <InputBlock title="Max Surge" help="Установлено значение по-умолчанию">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Quick Shutdown">
              <InputSwitch />
            </InputBlock>
          </div>
        </div>
        <div class="mx-24 my-6">
          <CardTitle title="Параметры обновлений, влияющие на простой" />
          <div class="flex flex-col gap-y-6">
            <SelectButton v-model="updateMode" :options="updateModeOptions" optionLabel="name" optionValue="value" />
            
            <InputRow>
              <MultiSelect placeholder="Выберите дни" />
              <FormLabel value="С" />
              <Calendar :showTime="true" :timeOnly="true" />
              <FormLabel value="До" />
              <Calendar :showTime="true" :timeOnly="true" />
              <Button icon="pi pi-times" class="p-button-rounded p-button-danger p-button-outlined" />
            </InputRow>
            <Button label="Добавить" class="p-button-outlined p-button-info w-full" />

            <InputBlock title="Выгон подов с узла перед выдачей разрешения" tooltip="Тут будет тултип" type="wide">
              <InputSwitch class="shrink-0" />
            </InputBlock>
          </div>
        </div>
      </div>

      <CardDivider />
      
      <CardTitle title="Системные параметры узлов" />
      
      <div class="flex flex-wrap items-start -mx-24 -my-6">

        <div class="mx-24 my-6">
          <FieldGroupTitle title="Настройки CRI" tooltip="Тут будет тултип" />
          <div class="flex flex-col gap-y-12 mb-12">
            <SelectButton v-model="CRIMode" :options="CRIModeOptions" optionLabel="name" optionValue="value" />
            <InputBlock title="Максимальное количество параллельных потоков загрузки для каждой операции pull">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Автоматическое управление версией и параметрами Docker" tooltip="Тут будет тултип">
              <InputSwitch class="shrink-0" />
            </InputBlock>
          </div>

          <FieldGroupTitle title="Управление ядром" tooltip="Тут будет тултип" />
          <InputBlock title="Deckhouse управляет ядром ОС">
            <InputSwitch />
          </InputBlock>
        </div>

        <div class="mx-24 my-6">
          <FieldGroupTitle title="Параметры kubelet" tooltip="Тут будет тултип" />
          <div class="flex flex-col gap-y-12 mb-12">
            <InputBlock title="Максимальное количество файлов журналов с учетом ротации" spec="spec.kubelet.containerlogtyrpyr">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Максимальный размер файла журнала до ротации, МиБ" spec="spec.kubelet.containerlogtyrpyr">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Максимальное количество pod-ов" help="Установлено значение по-умолчанию">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
            <InputBlock title="Путь к каталогу для файлов kubelet">
              <InputText class="p-inputtext-sm w-[200px]" />
            </InputBlock>
          </div>
        </div>

      </div>

      <CardDivider />

      <CardTitle title="Параметры chaos monkey" />
      <div class="flex flex-col gap-y-6 mb-12">
        <SelectButton v-model="monkey" :options="monkeyOptions" optionLabel="name" optionValue="value" />      
        <InputBlock title="Интервал срабатывания" help="Установлено значение по-умолчанию">
          <Calendar :showTime="true" :timeOnly="true" class="w-[100px]" />
        </InputBlock>
      </div>

      </template>
      <template #actions>
          <ButtonBlock title="Удалить" type="danger-subtle"></ButtonBlock>
      </template>
      <template #notice>
          Scary error message! You have been warned! Scary error message! You have been warned! Scary error message! You have been warned! Scary error message! You have been warned
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions compact="false" />
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';

import Checkbox from 'primevue/checkbox';
import InputText from 'primevue/inputtext';
import SelectButton from "primevue/selectbutton";
import Button from 'primevue/button';
import InputSwitch from 'primevue/inputswitch';
import MultiSelect from 'primevue/multiselect';
import Calendar from 'primevue/calendar';

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import ButtonBlock from "@/components/common/button/ButtonBlock.vue";

import FormActions from "@/components/common/form/FormActions.vue";
import InputLabelGroup from "@/components/common/form/InputLabelGroup.vue";
import InputBlock from "@/components/common/form/InputBlock.vue";
import InputRow from "@/components/common/form/InputRow.vue";
import FieldGroupTitle from "@/components/common/form/FieldGroupTitle.vue";
import FormLabel from "@/components/common/form/FormLabel.vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardTitle from "@/components/common/card/CardTitle.vue";
import CardDivider from "@/components/common/card/CardDivider.vue";
import CardParamGrid from "@/components/common/card/CardParamGrid.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import CardParamGroup from "@/components/common/card/CardParamGroup.vue";
import CardParamGroupItem from "@/components/common/card/CardParamGroupItem.vue";
import CardTable from "@/components/common/card/CardTable.vue";

const updateMode = ref('auto');
const updateModeOptions = [{ name: 'Авто', value: 'auto'}, { name: 'Rolling updates', value: 'rolling'}, { name: 'Ручной', value: 'manual'}];

const CRIMode = ref('docker');
const CRIModeOptions = [{ name: 'Docker', value: 'docker'}, { name: 'Containerd', value: 'containerd'}, { name: 'Not managed', value: 'not_managed'}];

const monkey = ref('dnd');
const monkeyOptions = [{ name: 'Disabled', value: 'disabled'}, { name: 'Drain and delete', value: 'dnd'}];

const interval = ref('6h');
const intervalOptions = [{ name: '6 часов', value: '6h'}, { name: '30 минут', value: '30m'}, { name: '2 часа 30 минут', value: '2h30m'}];

const table_fields = [
  "Created at",
  "Type",
  "Reason",
  "From",
  "Message"
]

const table_data = [
  {
    "created_at": "19:00",
    "type": "Normal",
    "reason": "Node not ready",
    "from": "kubelet",
    "message": "Node status is now: node not ready"
  },
  {
    "created_at": "19:00",
    "type": "Normal",
    "reason": "Node not ready",
    "from": "kubelet",
    "message": "Node status is now: node not ready"
  },
  {
    "created_at": "19:00",
    "type": "Normal",
    "reason": "Node not ready",
    "from": "kubelet",
    "message": "Node status is now: node not ready"
  },
  {
    "created_at": "19:00",
    "type": "Normal",
    "reason": "Node not ready",
    "from": "kubelet",
    "message": "Node status is now: node not ready"
  },
  {
    "created_at": "19:00",
    "type": "Normal",
    "reason": "Node not ready",
    "from": "kubelet",
    "message": "Node status is now: node not ready"
  },
]

const tabs = [
  {
    id: "1",
    title: "Просмотр",
    routeName: "home",
  },
  {
    id: "2",
    title: "Редактирование",
    active: true,
    routeName: "home",
  },
];
</script>
