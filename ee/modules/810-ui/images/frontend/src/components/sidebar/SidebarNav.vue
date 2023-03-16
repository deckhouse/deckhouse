<template>
  <nav class="hs-accordion-group py-6 w-full flex flex-col flex-wrap" data-hs-accordion-always-open>
    <ul>
      <SidebarNavItem v-for="item in items" :key="item.id" :item="item" />
    </ul>
  </nav>
</template>

<script setup lang="ts">
import SidebarNavItem from "./SidebarNavItem.vue";
import { watch, reactive } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();

const items = reactive([
  {
    id: "1",
    title: "Обновления",
    icon: "IconOverview",
    active: false,
    routeNames: ["home", "IndexSettings"],
  },
  {
    id: "2",
    title: "Управление узлами",
    icon: "IconOverview",
    active: false,
    routeNames: ["NodeGroupList"],
    children: [
      {
        id: "2_1",
        title: "Группы узлов",
        icon: "IconOverview",
      },
      {
        id: "2_2",
        title: "Классы машин",
        icon: "IconOverview",
      },
    ],
  },
]);

watch(
  () => route.name,
  (newVal) => {
    var oldActive = items.find((i) => {
      return i.active === true;
    });
    if (oldActive) oldActive.active = false;
    var newActive = items.find((i) => {
      return i.routeNames.indexOf(String(newVal)) > -1;
    });
    if (newActive) newActive.active = true;
  },
  {
    immediate: true,
    flush: "post",
  }
);
</script>
