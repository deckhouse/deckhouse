<template>
  <nav class="hs-accordion-group py-6 w-full flex flex-col flex-wrap" data-hs-accordion-always-open>
    <ul>
      <SidebarNavItem v-for="(item, idx) in items" :key="idx" :item="item" />
    </ul>
  </nav>
</template>

<script setup lang="ts">
import SidebarNavItem from "./SidebarNavItem.vue";
import { computed } from "vue";

import useLoadAll from "@/composables/useLoadAll";

const { lists } = useLoadAll();

const items = computed(() => [
  {
    title: "Обновления",
    icon: "IconOverview",
    routeNames: ["Home", "DeckhouseSettings"],
    badge: lists.releases && lists.releases.items.length,
  },
  {
    title: "Управление узлами",
    icon: "IconOverview",
    routeNames: ["NodeGroupList"],
    children: [
      {
        title: "Группы узлов",
        routeNames: ["NodeGroupList", "NodeGroupShow", "NodeGroupEdit", "NodeShow", "NodeEdit", "NodeList"],
        badge: lists.nodeGroups && lists.nodeGroups.items.length,
      },
      {
        title: "Классы машин",
        routeNames: ["InstanceClassesList", "InstanceClassShow", "InstanceClassEdit", "InstanceClassNew"],
        badge: lists.instanceClasses && lists.instanceClasses.items.length,
      },
      {
        title: "Узлы всех групп",
        routeNames: ["NodeListAll"],
        badge: lists.nodes && lists.nodes.items.length,
      },
    ],
  },
]);
</script>
