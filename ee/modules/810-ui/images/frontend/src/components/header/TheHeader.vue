<template>
  <header class="sticky top-0 inset-x-0 flex flex-row flex-wrap z-[48] w-full bg-white border-b text-sm py-5 pl-64 pr-3">
    <nav class="flex items-center px-10" aria-label="Global">
      <div class="w-full flex items-center justify-end ml-auto justify-between gap-x-3 order-3">
        <Breadcrumb v-if="route.meta.breadcrumbs" :model="route.meta.breadcrumbs(route)" class="!p-0 !border-0">
          <template #item="item">
            <router-link :to="item.item.to || '/'" :class="item.item.active ? 'text-slate-800' : 'text-slate-400'">
              {{ item.item.label }}
            </router-link>
          </template>
        </Breadcrumb>
      </div>
    </nav>
    <div class="ml-auto flex items-center">
      <span class="block font-medium text-gray-800 mr-3">Spec</span>
      <InputSwitch v-model="specMode" inputClass="test" />
    </div>
  </header>
</template>

<script setup lang="ts">
import { useRoute } from "vue-router";
import useSpecSwitch from "@/composables/useSpecMode";

import Breadcrumb from "primevue/breadcrumb";
import InputSwitch from "primevue/inputswitch";

const route = useRoute();

const { specMode } = useSpecSwitch();
</script>
