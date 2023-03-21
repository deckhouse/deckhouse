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

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <InputBlock title="Имя" type="column" class="mb-6" required>
              <InputText class="p-inputtext-sm w-[450px]" />
            </InputBlock>
            <FieldGroupTitle title="Диск" />
            <div class="flex flex-col gap-y-6">
              <InputBlock title="Размер ГБ" spec="spec.rootDiskSizeGb" help="Этот параметр влияет на тип диска<br> <a href='ya.ru' target='_blank' class='text-blue-500'>Как подобрать размер диска</a>">
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Сеть" />

            <div class="flex flex-col gap-y-6">
              <InputBlock title="Основная сеть" spec="spec.mainSubnet" type="column">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputLabelGroup title="Дополнительные сети" spec="spec.additionalNetworks" :fields="['Network']" />
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Прочее" />

            <div class="flex flex-col gap-y-6">
              <InputBlock title="Образ машины" spec="spec.ImageName" type="column" help="Установлено значение по-умолчанию">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
            </div>
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <InputLabelGroup class="mx-12 my-6" title="Дополнительные группы безопасности" spec="spec.extraSecurityGroups" :fields="['Value']" default="Настройки провайдера" />
          <InputLabelGroup class="mx-12 my-6" title="Дополнительные теги" spec="spec.additionalLabels" :fields="['Key', 'Value']" default="Настройки провайдера" />
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
import SelectButton from "primevue/selectbutton";

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

const tabs = [
  {
    title: "Просмотр",
    routeName: "Home",
  },
  {
    title: "Редактирование",
    routeName: "Home",
  },
];
</script>
