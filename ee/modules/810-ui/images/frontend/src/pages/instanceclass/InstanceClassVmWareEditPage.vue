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
        <CardTitle title="Конфигурация" icon="IconVmWareLogo" />
      </template>
      <template #content>

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <FieldGroupTitle title="Ресурсы" />
            <div class="flex flex-col gap-y-6">
              <InputBlock title="ЦПУ, виртуальных ядер" spec="spec.numCPUs" required>
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
              <InputBlock title="Память МмБ" spec="spec.memory">
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
              <InputBlock title="Размер корневого диска Гб" spec="spec.rootDiskSize" help="Установлено значение по-умолчанию">
                <InputText class="p-inputtext-sm w-[100px]" />
              </InputBlock>
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Сеть" />

            <div class="flex flex-col gap-y-6">
              <InputBlock title="Основная сеть" spec="spec.mainNetwork" type="column">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputLabelGroup title="Дополнительные подсети" spec="spec.additionalSubnets" :fields="['Network']" />
            </div>
          </div>

          <div class="mx-24 my-6">
            <FieldGroupTitle title="Прочее" />

            <div class="flex flex-col gap-y-6">
              <InputBlock title="Идентификатор образа" spec="spec.template" type="column" help="Установлено значение по-умолчанию">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputBlock title="Datastore" spec="spec.datastore" type="column" help="Установлено значение по-умолчанию">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputBlock title="Пул ресурсов" spec="spec.resourcePool" type="column" help="Установлено значение из конфигурации провайдера">
                <InputText class="p-inputtext-sm w-[450px]" />
              </InputBlock>
              <InputBlock title="Включить синхронизацию времени с ESXi в гостевой ОС" spec="spec.disableTimesync" special>
                <InputSwitch />
              </InputBlock>
            </div>
          </div>
        </div>

        <CardDivider />

        <CardTitle title="Дополнительные параметры" />

        <div class="flex flex-wrap items-start -mx-24 -my-6">
          <div class="mx-24 my-6">
            <div class="flex flex-col gap-y-6">
              <InputBlock title="Верхний лимит потребляемой частоты ЦПУ, МГц" spec="spec.runtimeOptions.cpuLimit" toggle>
                <InputText class="p-inputtext-sm w-[50px]" />
              </InputBlock>
              <InputBlock title="Величина зарезервированной потребляемой частоты ЦПУ, МГц" spec="spec.runtimeOptions.cpuReservation" toggle>
                <InputText class="p-inputtext-sm w-[50px]" />
              </InputBlock>
              <InputBlock title="Относительная величина CPU Shares" spec="spec.runtimeOptions.cpuShares" toggle>
                <InputText class="p-inputtext-sm w-[50px]" />
              </InputBlock>
              <InputBlock title="Верхний лимит потребляемой памяти Миб" spec="spec.runtimeOptions.memoryLimit" toggle>
                <InputText class="p-inputtext-sm w-[50px]" />
              </InputBlock>
              <InputBlock title="Процент зарезервированной памяти в кластере ( % от spec.memory)" spec="spec.runtimeOptions.memoryReservations" help="Допустимые значения 0 < x < 100<br>Значение по умолчанию: 80" toggle>
                <InputText class="p-inputtext-sm w-[50px]" />
              </InputBlock>
              <InputBlock title="Относительная величина Memory Shares" spec="spec.runtimeOptions.memoryShares" help="Допустимые значения 0 < x < 100" toggle>
                <InputText class="p-inputtext-sm w-[50px]" />
              </InputBlock>
            </div>
          </div>

          <div class="mx-24 my-6">
            <InputBlock title="Hardware Assisted Virtualization" spec="spec.runtimeOptions.nestedHardwareVirtualisation" special class="w-[550px]">
              <InputSwitch />
            </InputBlock>
          </div>
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
