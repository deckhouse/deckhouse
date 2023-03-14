<template>
  <CardBlock title="The-node-group-name" notice-type="warning" url="#" :badges="[ { id: 1, title: 'Порядочек', type: 'success'} ]">
    <template #content>      
      <div class="flex flex-wrap items-start gap-x-12 gap-y-6 mb-6">
        <CardParamGroup title="Состояние узлов">
          <CardParamGroupItem title="Всего узлов" value="6" />
          <CardParamGroupItem title="Готовые" value="6" state="danger" />
          <CardParamGroupItem title="Актуальные" value="5" />
        </CardParamGroup>

        <CardParamGroup title="Параметры автомасштабирования">
          <CardParamGroupItem title="Узлов на зону" value="3–10" />
          <CardParamGroupItem title="Необходимо" value="7" />
          <CardParamGroupItem title="Заказано" value="6" />
          <CardParamGroupItem title="Резерв" value="2/6" />
        </CardParamGroup>
      </div>

      <CardParamGrid>
        <CardParam title="Зоны" :value="['eu-west-1a', 'eu-west-1b', 'eu-west-1c']" />
        <CardParam title="Класс машин" value="Yandex.Cloud" />
        <CardParam title="Тип узлов" value="Cloud Ephemeral" />
        <CardParam title="Приоритет групппы" value="10" />
        <CardParam title="Версия Kubernetes" value="1.20.34" />
      </CardParamGrid>

      <div class="flex flex-wrap items-start gap-x-3 gap-y-2 mt-6">
        <CardLabel title="The-biggest-node-group-name-imaginable-is-63-symbols-max-woohoo" />
        <CardLabel title="app: warp-drive-ai" />
        <CardLabel title="environment: production" />
        <CardLabel title="environment: production" />
        <CardLabel title="app: warp-drive-ai" />
      </div>
    </template>
    <template #actions>
      <ButtonBlock title="Редактировать" type="primary"></ButtonBlock>
      <ButtonBlock title="Удалить" type="danger-subtle"></ButtonBlock>
    </template>
    <template #notice>
      Scary error message! You have been warned! Scary error message! You have been warned! Scary error message! You have been warned! Scary error message! You have been warned
    </template>
  </CardBlock>
</template>

<script setup lang="ts">
import ButtonBlock from "../common/button/ButtonBlock.vue";

import CardBlock from "../common/card/CardBlock.vue";
import CardDivider from "../common/card/CardDivider.vue";
import CardParamGrid from "../common/card/CardParamGrid.vue";
import CardParam from "../common/card/CardParam.vue";
import CardParamGroup from "../common/card/CardParamGroup.vue";
import CardParamGroupItem from "../common/card/CardParamGroupItem.vue";
import CardLabel from "../common/card/CardLabel.vue";
</script>