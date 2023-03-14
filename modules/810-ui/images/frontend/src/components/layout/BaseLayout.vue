<template>
  <div class="layout flex flex-col min-w-[1024px] text-gray-800">
    <TheHeader />
    <TheSidebar />

    <div class="w-full pt-10 px-4 pl-72 flex-1 relative" :class="compact == true ? 'max-w-screen-2xl' : ''">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import TheHeader from "../header/TheHeader.vue";
import TheSidebar from "../sidebar/TheSidebar.vue";
const props = defineProps({
  compact: {
    type: Boolean,
    default: true,
  },
});
</script>

<style scoped>
.layout {
  background: #f0f9ff;
  min-height: 100vh;
}
</style>
