<template>
  <div
    class="rounded-md overflow-hidden bg-white transition-all flex items-stretch drop-shadow-[0_0_3px_rgba(0,0,0,0.1)] hover:drop-shadow-[0_0_6px_rgba(0,0,0,0.15)]"
  >
    <div class="flex-1">
      <div
        class="flex gap-3 items-center shadow-inner px-6 py-3"
        :class="getNoticeStyles(noticeType)"
        v-if="$slots.notice && noticePlacement == 'top'"
      >
        <component :is="Icons['IconInfo']" />
        <div><slot name="notice" /></div>
      </div>
      <div class="p-6">
        <div class="flex justify-between">
          <slot name="title">
            <CardTitle v-if="title" :title="title" :badges="badges" :route="route" />
          </slot>
          <div class="flex gap-3 items-start">
            <slot name="actions" />
          </div>
        </div>
        <slot name="content" v-if="!contentLoading" />
        <div v-else>
          <Skeleton class="mb-2"></Skeleton>
          <Skeleton width="10rem" class="mb-2"></Skeleton>
        </div>
        <div v-if="icon" class="absolute right-6 bottom-6">
          <component :is="Icons[icon]" />
        </div>
      </div>
      <div
        class="flex gap-3 items-center shadow-inner px-6 py-3"
        :class="getNoticeStyles(noticeType)"
        v-if="$slots.notice && noticePlacement == 'bottom'"
      >
        <component :is="Icons['IconInfo']" />
        <div><slot name="notice" /></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import type { IBadge, IconsType } from "@/types";

import * as Icons from "@/components/common/icon";
import CardTitle from "@/components/common/card/CardTitle.vue";
import Skeleton from "primevue/skeleton";

const props = defineProps({
  title: String,
  tooltip: String,
  badges: Array as PropType<Array<IBadge>>,
  noticeType: {
    type: String,
    default: "default",
  },
  noticePlacement: {
    type: String,
    default: "bottom",
  },
  icon: String as PropType<IconsType>,
  route: [String, Object],
  contentLoading: {
    type: Boolean,
    default: false,
  },
});

function getNoticeStyles(notice_type: string | undefined): string {
  const classes = {
    default: "bg-slate-50 text-slate-500",
    warning: "bg-orange-100 text-orange-500",
    danger: "bg-red-100 text-red-500",
  };
  return notice_type ? classes[notice_type as keyof typeof classes] : classes["default"];
}
</script>
