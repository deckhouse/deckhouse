<template>
  <PageTitle>The-biggest-node-group-name-imaginable-is-63-symbols-max-woohoo</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <GridBlock>
    <CardBlock notice-placement="top">
      <template #title>
        <CardTitle title="Конфигурация" icon="IconOpenStackLogo" />
      </template>
      <template #content>
        <TabsBlock :items="card_tabs" class="mb-10" />
        
        <InputBlock title="Тип Машины" spec="spec.diskSizeGb" help="Обязательный параметр">
          <Dropdown :options="typeOptions" v-model="type" optionLabel="" class="p-inputtext-sm" />
        </InputBlock>

        <CardDivider />

        <FieldGroupTitle title="Диск" />
        <div class="flex flex-wrap items-start -mx-24 -my-6">          
          <div class="mx-24 my-6">
            <InputBlock title="Размер ГБ" spec="spec.diskSizeGb" 
              help="Параметр влияет на тип диска.<br><a href='ya.ru' target='_blank' class='text-blue-500'>Прочитайте</a>, как подобрать размер диска">
              <InputText class="p-inputtext-sm w-[50px]" />
            </InputBlock>
          </div>
          <div class="mx-24 my-6"> 
            <InputBlock title="Образ машины" spec="spec.diskSizeGb" help="По умолчанию используется значение из настроек провайдера">
              <InputText class="p-inputtext-sm" />
            </InputBlock>
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <FieldGroupTitle title="Основная сеть" />
            <InputBlock type="column" help="По-умолчанию используется значение из настроек провайдера">
              <InputText class="p-inputtext-sm w-full" />
            </InputBlock>
          </div>
          <InputLabelGroup class="mx-24 my-6" title="Дополнительные сети" :fields="['Value']" />
        </div>

        <CardDivider />
        
        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <InputLabelGroup class="mx-12 my-6" title="Дополнительные группы безопасности" spec="spec.extraSecurityGroups" :fields="['Value']" default="Настройки провайдера" />
          <InputLabelGroup class="mx-12 my-6" title="Дополнительные теги" spec="spec.extraTags" :fields="['Key', 'Value']" default="Настройки провайдера" />
        </div>

      </template>
      <template #actions>
          <ButtonBlock title="Удалить" type="danger-subtle"></ButtonBlock>
      </template>
      <template #notice>
          Используется в 3 группах узлов: <b>big-node-group, redis, oopyachka-node</b>
      </template>
    </CardBlock>
  </GridBlock>
  <FormActions compact="false" />
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';

import Dropdown from 'primevue/dropdown';
import InputText from 'primevue/inputtext';
import Button from 'primevue/button';
import InputSwitch from 'primevue/inputswitch';

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import ButtonBlock from "@/components/common/button/ButtonBlock.vue";

import FormActions from "@/components/common/form/FormActions.vue";
import InputBlock from "@/components/common/form/InputBlock.vue";
import InputLabelGroup from "@/components/common/form/InputLabelGroup.vue";
import FieldGroupTitle from "@/components/common/form/FieldGroupTitle.vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardTitle from "@/components/common/card/CardTitle.vue";
import CardDivider from "@/components/common/card/CardDivider.vue";

const type = ref('m5.xlarge');
const typeOptions = ref(['m5.xlarge']);

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

const card_tabs = [
  {
    id: "1",
    title: "Стандартная",
    active: true,
    routeName: "home",
  },
  {
    id: "2",
    title: "Пользовательская",
    routeName: "home",
  },
];
</script>
