<template>
  <div>
    <div v-if="!isLoading">
      <CardBlock title="Текущий режим обновлений">
        <template #content>
          <div class="grid grid-cols-3 gap-5">
            <CardParam type="col" title="Режим обновлений" :value="deckhouseSettings?.spec.settings.update.mode" />
            <CardParam type="col" title="Канал" :value="deckhouseSettings?.spec.settings.releaseChannel" />
            <CardParam type="col" title="Окна обновлений">
              <span v-for="(w, index) in deckhouseSettings?.spec.settings.update.windows" :key="index">
                с {{ w.from }} до {{ w.to }} — {{ w.days.join(", ") }}
                <br v-if="index < deckhouseSettings?.spec.settings.update.windows.length" />
              </span>
            </CardParam>
          </div>
        </template>
        <template #notice> Время всех обновлений на странице указано в часовом поясе вашего браузера: <b>UTC+05:30</b> </template>
      </CardBlock>
    </div>
    <div v-if="isLoading">LOADING...</div>
  </div>
</template>
<script setup lang="ts">
import DeckhouseModuleSettings, { DeckhouseSettings } from "@/models/DeckhouseModuleSettings";
import { ref } from "vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import { watch, computed, getCurrentInstance } from "vue";

const deckhouseSettings = ref<DeckhouseSettings>();
const isLoading = ref(true);

/*
const settings = computed(() => {
  return deckhouseSettings.spec ? deckhouseSettings.spec.settings : undefined;
})
*/

DeckhouseModuleSettings.get().then((res: any) => {
  deckhouseSettings.value = res;
  isLoading.value = false;

  // @ts-ignore
  DeckhouseModuleSettings.subscribe();
});
</script>
