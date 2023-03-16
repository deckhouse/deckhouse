<template>
  <CardBlock :title="item.metadata.name" :badges="getBadges(item)">
    <template #content>
      <div class="flex items-start space-x-6 justify-between">
        <div class="flex items-start space-x-6">
          <CardParam type="col" title="Дата релиза" :value="item.metadata.creationTimestamp" />
          <CardParam type="col" title="Дата установки" :value="item.status.transitionTime" v-if="item.status.transitionTime" />
        </div>
      </div>
    </template>
    <template #actions>
      <ButtonBlock title="Установить обновление" type="primary" icon="IconInstall" v-if="item.status.phase == 'Pending'"></ButtonBlock>
      <ButtonBlock title="Читать Changelog" type="subtle" @click="openChangelog(item)"></ButtonBlock>
    </template>
    <template #notice v-if="item.status.phase == 'Pending'"> Обновление готово к установке </template>
  </CardBlock>
</template>
<script setup lang="ts">
import type DeckhouseRelease from "@/models/DeckhouseRelease";
import CardBlock from "../common/card/CardBlock.vue";
import CardParam from "../common/card/CardParam.vue";
import ButtonBlock from "../common/button/ButtonBlock.vue";

const props = defineProps({
  item: {
    type: Object as () => DeckhouseRelease,
    required: true,
  },
});
const emit = defineEmits(["requestChangelog"]);

function openChangelog(item: DeckhouseRelease) {
  emit("requestChangelog", item);
}

function getBadges(item: DeckhouseRelease) {
  let badges = [
    {
      id: 1,
      title: item.status.phase,
      styles: getStatusStyles(item),
    },
  ];

  if (item.status.phase == "Deployed") {
    badges.push({
      id: 2,
      title: "Текущая версия",
      styles: getStatusStyles(item),
    });
  }

  return badges;
}

function getStatusStyles(item: DeckhouseRelease) {
  let styles = "";

  if (item.status.phase == "Pending") {
    styles = "bg-yellow-500";
  } else if (item.status.phase == "Deployed") {
    styles = "bg-green-500";
  } else {
    styles = "bg-blue-300";
  }

  return styles;
}
</script>
