<template>
  <CardBlock :title="item.metadata.name" :badges="getBadges()" notice-type="warning">
    <template #content>
      <CardParamGrid>
        <CardParam title="Дата релиза" :value="formatTime(item.metadata.creationTimestamp)" />
        <CardParam title="Дата установки" :value="formatTime(item.status.transitionTime)" v-if="item.status.transitionTime" />
      </CardParamGrid>
    </template>
    <template #actions>
      <ButtonBlock title="Установить обновление" type="primary" icon="IconInstall" @click="approve()" v-if="canApprove()"></ButtonBlock>
      <ButtonBlock title="Читать Changelog" type="subtle" @click="toggleChangelog()"></ButtonBlock>
    </template>
    <template #notice v-if="item.status.phase == 'Pending'"> Обновление готово к установке </template>
  </CardBlock>
</template>
<script setup lang="ts">
import { formatTime } from "@/utils";

import type DeckhouseRelease from "@/models/DeckhouseRelease";
import CardBlock from "@/components/common/card/CardBlock.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import CardParamGrid from "@/components/common/card/CardParamGrid.vue";
import ButtonBlock from "@/components/common/button/ButtonBlock.vue";
// import { watch, reactive, getCurrentInstance } from 'vue';

const props = defineProps({
  item: {
    type: Object as () => DeckhouseRelease,
    required: true,
  },
});
const emit = defineEmits(["toggleChangelog"]);

function toggleChangelog() {
  emit("toggleChangelog", props.item);
}

function getBadges() {
  let badges = [
    {
      title: props.item.status.phase,
      type: getStatusType(),
    },
  ];

  if (props.item.status.phase == "Deployed") {
    badges.push({
      title: "Текущая версия",
      type: getStatusType(),
    });
  }

  return badges;
}

function getStatusType() {
  let styles = "";

  if (props.item.status.phase == "Pending") {
    styles = "warning";
  } else if (props.item.status.phase == "Deployed") {
    styles = "success";
  } else {
    styles = "info";
  }

  return styles;
}

function canApprove() {
  return props.item.status.phase == "Pending";
}

function approve() {
  return props.item
    .approve()
    .then((resp: any) => {
      console.log("TODO: DeckhouseRelease.approve ERROR FLASH-MESSAGE");
    })
    .catch((err: any) => {
      console.error("TODO: DeckhouseRelease.approve ERROR FLASH-MESSAGE");
    });
}
</script>
