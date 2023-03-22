<template>
  <template v-if="!list.isLoading.value">
    <ReleaseItem v-for="item in list.items" :key="item.metadata.name" :item="item" @toggle-changelog="toggleChangelogWindow" />
  </template>
  <CardBlock v-if="list.isLoading.value" :content-loading="true"></CardBlock>
  <CardEmpty v-if="!list.isLoading.value && list.items.length == 0" />
  
  <Sidebar :header="popup.title" position="right" v-model:visible="popup.key" class="p-sidebar-md" :modal="false">
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
import { reactive, ref, onBeforeUnmount } from "vue";

import DeckhouseRelease from "@/models/DeckhouseRelease";
import ReleaseItem from "@/components/releases/ReleaseItem.vue";
import useListDynamic from "@lib/nxn-common/composables/useListDynamic";
import Sidebar from "primevue/sidebar";
import CardBlock from "@/components/common/card/CardBlock.vue";
import CardEmpty from "@/components/common/card/CardEmpty.vue";

const emit = defineEmits<{ (e: "set-count", value: number): void }>();
function resetCount() { console.log(`deckhousereleases.deckhouse.io: emit("set-count", ${list.items.length});`); emit("set-count", list.items.length); }
const filter = reactive({});
const list = useListDynamic<DeckhouseRelease>(
  DeckhouseRelease,
  {
    onLoadSuccess: resetCount,
    afterAdd: resetCount,
    afterRemove: resetCount,

    sortBy: (a: DeckhouseRelease, b: DeckhouseRelease) => {
      return Date.parse(b.metadata.creationTimestamp) - Date.parse(a.metadata.creationTimestamp);
    },

    onLoadError: (error: any) => {
      console.error("Failed to load counts: " + JSON.stringify(error));
    },
  },
  filter
);

list.activate();

// popups
const popup = reactive({
  title: "",
  content: "",
  link: "",
  key: null,
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

onBeforeUnmount(() => list.destroyList() );
</script>
