<template>
  <template v-if="!isLoading">
    <ReleaseItem v-for="item in lists.releases.items" :key="item.metadata.name" :item="item" @toggle-changelog="toggleChangelogWindow" />
  </template>
  <CardBlock v-if="isLoading" :content-loading="true"></CardBlock>
  <CardEmpty v-if="!isLoading && lists.releases.items.length == 0" />

  <Sidebar :header="popup.title" position="right" :visible="!!popup.key" class="p-sidebar-md" :modal="false">
    <span class="block text-2xl font-medium text-gray-800 mb-6">Changelog: {{ popup.title }}</span>
    <div v-for="(cl_value, cl_label) in popup.content" :key="cl_label" class="mb-6">
      <span class="block text-lg font-medium text-gray-800 mb-1">{{ cl_label }}</span>
      <span v-for="(fixes_value, fixes_label) in cl_value.fixes" :key="fixes_label" class="block text-gray-800 mb-3">
        {{ fixes_value.summary }}
      </span>
    </div>
    <a :href="popup.link" target="_blank" class="text-blue-500 underline">{{ popup.link }}</a>
  </Sidebar>
</template>

<script setup lang="ts">
import { reactive } from "vue";

import type DeckhouseRelease from "@/models/DeckhouseRelease";
import ReleaseItem from "@/components/releases/ReleaseItem.vue";
import Sidebar from "primevue/sidebar";
import CardBlock from "@/components/common/card/CardBlock.vue";
import CardEmpty from "@/components/common/card/CardEmpty.vue";
import useLoadAll from "@/composables/useLoadAll";
import type { DeckhouseReleaseChangelog } from "@/models/DeckhouseRelease";

const { lists, isLoading } = useLoadAll();

// popups
const popup = reactive({
  title: "",
  content: {} as DeckhouseReleaseChangelog,
  link: "",
  key: null as null | string,
});

function toggleChangelogWindow(data: DeckhouseRelease) {
  var newKey = data.primaryKey();
  if (popup.key == newKey) {
    popup.key = null;
  } else {
    popup.title = data.spec.version;
    popup.content = data.spec.changelog;
    popup.link = data.spec.changelogLink;
    popup.key = newKey;
  }
}
</script>
