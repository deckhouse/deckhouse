<template>
    <div>
      <div class="text-slate-800 leading-normal">
        {{ title }}
        <span class="text-red-500" v-if="required" v-tippy="'Обязательное поле'">*</span>
        <div v-if="tooltip" class="text-slate-400 ml-1 inline">
          <component :is="Icons['IconInfo']" v-tippy="tooltip" class="inline"/>
        </div>
      </div>
      <SpecLabel :spec="spec" v-if="spec" />
    </div>
</template>

<script setup lang="ts">
import * as Icons from "@/components/common/icon";
import SpecLabel from "@/components/common/form/SpecLabel.vue";
const props = defineProps({
  title: String,
  spec: String,
  tooltip: String,
  required: Boolean
});
</script>