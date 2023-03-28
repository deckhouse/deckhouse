<template>
  <div>
    <CardBlock :content-loading="isLoading">
      <template #content v-if="!isLoading">
        <CardParamGrid>
          <CardParam
            title="Режим обновлений"
            :value="deckhouseSettings?.spec.settings.release ? deckhouseSettings?.spec.settings.release.mode : '—'"
          />
          <CardParam title="Канал обновлений" :value="deckhouseSettings?.spec.settings.releaseChannel" />
          <CardParam title="Окна обновлений" class="col-span-2">
            <span v-if="deckhouseSettings?.spec.settings.release">
              <span v-for="(w, index) in deckhouseSettings?.spec.settings.release.windows" :key="index">
                с {{ w.from }} до {{ w.to }} — {{ w.days.join(", ") }}
                <br v-if="index < deckhouseSettings?.spec.settings.release.windows.length" />
              </span>
            </span>
            <span v-else>—</span>
          </CardParam>
        </CardParamGrid>
      </template>
      <template #notice>
        Время всех обновлений на странице указано в часовом поясе вашего браузера:
        <b>{{ dayjs.tz.guess() }} ({{ dayjs().format("Z") }})</b>
      </template>
    </CardBlock>
  </div>
</template>
<script setup lang="ts">
import dayjs from "dayjs";
import DeckhouseModuleSettings from "@/models/DeckhouseModuleSettings";
import { ref } from "vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardParamGrid from "../common/card/CardParamGrid.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import { onBeforeUnmount } from "vue";
import useLoadAll from "@/composables/useLoadAll";

const { deckhouseSettings, isLoading } = useLoadAll();
</script>
