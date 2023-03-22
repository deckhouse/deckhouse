<template>
  <div class="layout flex flex-col min-w-[1024px] text-gray-800">
    <TheHeader />
    <TheSidebar />
    <FlashMessages />

    <div class="w-full pt-10 px-4 pl-72 flex-1 relative flex flex-col" :class="compact == true ? 'max-w-screen-2xl' : ''">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import TheHeader from "../header/TheHeader.vue";
import TheSidebar from "../sidebar/TheSidebar.vue";

import FlashMessages from "@/components/common/page/FlashMessages.vue";
import FlashMessagesService from "@/services/FlashMessagesService.js";
import mitt, { type Emitter } from "mitt";
import { ref, provide } from "vue";

const props = defineProps({
  compact: {
    type: Boolean,
    default: true,
  },
});

// TODO: separate emitter for FlashMessages with it's separate message types?
type Events = { [key: string]: any }; // TODO: do mitt types?
const $flashMessages: Emitter<Events> = mitt<Events>();
provide("$flashMessages", $flashMessages);
FlashMessagesService.$flashMessages = $flashMessages; // TODO: don't do it? pass as arg to FlashMessagesService everytime?
</script>

<style scoped>
.layout {
  background: #fefeff;
  min-height: 100vh;
}
</style>
