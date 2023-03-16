<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <GridBlock>
    <DeckhouseModuleSettings />

    <ReleaseItemsList @set-count="ReleaseItemsCount = $event" />
  </GridBlock>
</template>

<script setup lang="ts">
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import DeckhouseModuleSettings from "@/components/releases/DeckhouseModuleSettings.vue";
import ReleaseItemsList from "@/components/releases/ReleaseItemsList.vue";
import { ref } from "vue";
import type { ITabsItem } from "@/types";

const ReleaseItemsCount = ref<number>(0);

const tabs = [
  {
    id: "1",
    title: "Версии",
    active: true,
    badge: ReleaseItemsCount,
    routeName: "home",
  },
  {
    id: "2",
    title: "Настройки обновлений",
    routeName: "DeckhouseSettings",
  },
] as Array<ITabsItem>;
</script>
