<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <GridBlock>
    <DeckhouseModuleSettings />

    <ReleaseItemsList />
  </GridBlock>
</template>

<script setup lang="ts">
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import DeckhouseModuleSettings from "@/components/releases/DeckhouseModuleSettings.vue";
import ReleaseItemsList from "@/components/releases/ReleaseItemsList.vue";
import { ref, computed } from "vue";
import type { TabsItem } from "@/types";
import useLoadAll from "@/composables/useLoadAll";

const { lists } = useLoadAll();

const tabs = computed<TabsItem[]>(() => [
  {
    title: "Версии",
    badge: lists.releases.items.length,
    routeName: "Home",
  },
  {
    title: "Настройки обновлений",
    routeName: "DeckhouseSettings",
  },
]);
</script>
