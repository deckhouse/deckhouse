<template>
  <template v-if="!isLoading">
    <UpdateItem v-for="item in items" :key="item.metadata.name" :item="item" @request-changelog="openChangelogWindow" />
  </template>
  <CardBlock v-if="isLoading" :content-loading="isLoading"></CardBlock>

  <Dialog :header="popup.title" position="right" v-model:visible="popup.isDisplayed">
    {{ popup.content }}
    <br/>
    <a :href="popup.link" target="_blank">{{ popup.link }}</a>
  </Dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";

import DeckhouseRelease from "@/models/DeckhouseRelease";
import UpdateItem from "@/components/updates/UpdateItem.vue";
import Dialog from "primevue/dialog";
import CardBlock from "../common/card/CardBlock.vue";

const isLoading = ref(true);
const items: Array<DeckhouseRelease> = reactive([]);
const emit = defineEmits<{ (e: 'set-count', value: number): void }>()

DeckhouseRelease.query().then((resp) => {
  DeckhouseRelease.all().forEach((item:any) => items.push(item));
  resort();
  emit('set-count', items.length)
  isLoading.value = false;
});

function sortBy(a:any, b:any) {
  return Date.parse(b.metadata.creationTimestamp) - Date.parse(a.metadata.creationTimestamp);
}

function resort() {
  items.sort(sortBy);
}

// popups
const popup = reactive({
  title: "",
  content: "",
  link: "",
  isDisplayed: false
});

function openChangelogWindow(data: DeckhouseRelease) {
  popup.title = data.spec.version;
  popup.content = data.spec.changelog;
  popup.link = data.spec.changelogLink;
  popup.isDisplayed = true;
}

function closeChangelogWindow() {
  popup.isDisplayed = false;
}
</script>
