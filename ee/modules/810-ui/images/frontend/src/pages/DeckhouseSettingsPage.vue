<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <DeckhouseModuleSettingsForm :deckhouse-module-settings="deckhouseSettings" v-if="!isLoading && deckhouseSettings" />
  <CardBlock v-if="isLoading" :content-loading="isLoading" />
</template>

<script setup lang="ts">
import { ref, computed } from "vue";

import type { TabsItem } from "@/types";
import DeckhouseModuleSettings, { type IDeckhouseModuleRelease } from "@/models/DeckhouseModuleSettings";

import useLoadAll from "@/composables/useLoadAll";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";

import DeckhouseModuleSettingsForm from "@/components/releases/DeckhouseModuleSettingsForm.vue";

const { lists, deckhouseSettings, isLoading } = useLoadAll(({ deckhouseSettings }) => {
  // KOSTYL
  deckhouseSettings!.value!.spec.settings.release ||= {} as IDeckhouseModuleRelease;
  deckhouseSettings!.value!.spec.settings.release.notification ||= {};
});

const tabs = computed<TabsItem[]>(() => [
  {
    title: "Версии",
    badge: lists.releases.items?.length || null,
    routeName: "Home",
  },
  {
    title: "Настройки обновлений",
    routeName: "DeckhouseSettings",
  },
]);
</script>
