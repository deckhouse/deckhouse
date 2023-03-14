<template>
    <div>
      <div class="text-slate-800 leading-normal">
        {{ title }}
        <div v-if="tooltip" class="text-slate-400 ml-1 inline">
          <component :is="Icons['IconInfo']" v-tippy="tooltip" class="inline"/>
        </div>
      </div>
      <SpecLabel :spec="spec" v-if="spec" />
      <span class="text-sm text-slate-400 mt-1 leading-normal" v-html="help" v-if="help" />
    </div>
</template>

<script setup lang="ts">
import * as Icons from "@/components/common/icon";
import SpecLabel from "@/components/common/form/SpecLabel.vue";
const props = defineProps({
  title: String,
  help: String,
  spec: String,
  tooltip: String
});
</script>