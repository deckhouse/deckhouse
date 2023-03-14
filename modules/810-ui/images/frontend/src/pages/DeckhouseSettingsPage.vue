<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <DeckhouseModuleSettingsForm />
</template>

<script setup lang="ts">
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import DeckhouseModuleSettingsForm from "@/components/releases/DeckhouseModuleSettingsForm.vue";
import { ref } from "vue";
import type { ITabsItem } from "@/types";

const tabs = [
  {
    id: "1",
    title: "Версии",
    badge: ref<number>(0),
    routeName: "home",
  },
  {
    id: "2",
    active: true,
    title: "Настройки обновлений",
    routeName: "DeckhouseSettings",
  },
] as Array<ITabsItem>;
</script>
