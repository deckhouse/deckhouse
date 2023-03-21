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
        <CardTitle title="Конфигурация" icon="IconYandexCloudLogo" />
      </template>
      <template #content>
        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <FieldGroupTitle title="Ресурсы" />
            <div class="flex flex-col gap-y-6 mb-6">
              <InputBlock title="ЦПУ, виртуальных ядер" spec="spec.cores" required>
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
              <InputBlock
                title="Платформа ЦПУ"
                spec="spec.platformID"
                help="Установлено значение по умолчанию<br> <a href='ya.ru' target='_blank' class='text-blue-500'>Список существующих платформ</a>"
              >
                <InputText class="p-inputtext-sm" />
              </InputBlock>
              <InputBlock title="Память МмБ" spec="spec.memory">
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
              <InputBlock title="Базовый уровень производительности ядер" spec="spec.coreFraction" help="Установлено значение по-умолчанию">
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
              <InputBlock title="Количество графических адаптеров" spec="spec.gpus" help="Установлено значение по-умолчанию">
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
            </div>

            <FieldGroupTitle title="Диск" />
            <div class="flex flex-col gap-y-6">
              <InputBlock title="Размер ГБ" spec="spec.diskSizeGb">
                <InputText class="p-inputtext-sm" />
              </InputBlock>
              <InputBlock title="Тип" spec="spec.diskType">
                <Dropdown class="p-inputtext-sm" />
              </InputBlock>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Сеть" />

            <div class="flex flex-col gap-y-6">
              <InputBlock title="Основная подсеть" spec="spec.mainSubnet" type="column">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputBlock title="Тип сети" spec="spec.networkType">
                <Dropdown class="p-inputtext-sm" />
              </InputBlock>
              <InputLabelGroup title="Дополнительные подсети" spec="spec.additionalSubnets" :fields="['Network']" />
              <InputBlock title="Публичный IP" spec="spec.assignPublicIPAdress" special>
                <InputSwitch />
              </InputBlock>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Прочее" />

            <div class="flex flex-col gap-y-6">
              <InputBlock
                title="Идентификатор образа"
                spec="spec.ImageID"
                type="column"
                help="По умолчанию используется образ группы узлов master"
              >
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputBlock title="Прерываемые ВМ" spec="spec.preemptible" special>
                <InputSwitch />
              </InputBlock>
            </div>
          </div>
        </div>

        <CardDivider />

        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <InputLabelGroup class="mx-12 my-6" title="Дополнительные лейблы" spec="spec.additionalLabels" :fields="['Key', 'Value']" />
        </div>
      </template>
      <template #actions>
        <ButtonBlock title="Удалить" type="danger-subtle"></ButtonBlock>
      </template>
      <template #notice> Используется в 3 группах узлов: <b>big-node-group, redis, oopyachka-node</b> </template>
    </CardBlock>
  </GridBlock>
  <FormActions compact="false" />
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";

import Dropdown from "primevue/dropdown";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import InputSwitch from "primevue/inputswitch";
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
