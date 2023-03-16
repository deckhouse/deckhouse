<template>
  <div
    id="application-sidebar"
    class="fixed top-0 left-0 bottom-0 z-[60] w-64 bg-white border-r border-gray-200 pt-7 pb-10 overflow-y-auto scrollbar-y translate-x-0 right-auto bottom-0"
  >
    <div class="px-6">
      <a class="flex-none text-xl font-semibold dark:text-white" href="#" aria-label="Deckhouse Commander">
        <img alt="Deckhouse Commander" class="logo" src="@/assets/logo.svg" width="159" height="53" />
      </a>
    </div>
    <SidebarNav />
  </div>
</template>

<script setup lang="ts">
import SidebarNav from "./SidebarNav.vue";
</script>
