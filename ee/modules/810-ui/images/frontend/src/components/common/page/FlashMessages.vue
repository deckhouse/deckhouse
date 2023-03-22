<template>
  <div class="z-50 fixed bottom-0 left-1/2 -translate-x-1/2 flex flex-col gap-y-3">
    <div
      class="w-[800px] text-sm text-white rounded-md shadow-lg"
      :class="type_settings[message.type].styles"
      role="alert"
      v-for="message in messages"
      :key="message.key"
    >
      <div class="flex p-4 gap-x-2">
        <component :is="Icons[type_settings[message.type].icon]" class="h-4 w-4 mt-0.5" />

        <div>
          <span class="capitalize font-semibold">{{ message.type }}:</span> {{ message.text }}
        </div>

        <div class="ml-auto">
          <button
            type="button"
            v-if="!message.isReloader"
            @click="closeMessage(message.key)"
            class="inline-flex flex-shrink-0 justify-center items-center h-4 w-4 rounded-md text-white/[.5] hover:text-white focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-white/20 transition-all text-sm"
          >
            <span class="sr-only">Close</span>
            <component :is="Icons['IconClose']" class="h-4 w-4 mt-0.5" />
          </button>

          <button
            type="button"
            v-if="message.isReloader"
            @click="closeMessage(message.key)"
            :tooltip="'flash_messages.require_reload'"
            class="inline-flex flex-shrink-0 justify-center items-center h-4 w-4 rounded-md text-white/[.5] hover:text-white focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-white/20 transition-all text-sm"
          >
            <span class="sr-only">Close</span>
            <component :is="Icons['IconClose']" class="h-4 w-4 mt-0.5" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { watch, reactive, inject } from "vue";
import * as Icons from "@/components/common/icon";

import { type Emitter } from "mitt";
// TODO: separate emitter for FlashMessages with it's separate message types?
type Events = { [key: string]: any }; // TODO: do mitt types?
const $flashMessages = inject<Emitter<Events>>("$flashMessages");

type Message = {
  key: string;
  text: string;
  type: string;
  isReloader?: boolean;
  timeout?: number;
};
const messages = reactive<Array<Message>>([]);

$flashMessages.on("Flash::add", addMessage);
$flashMessages.on("Flash::close", closeKey);

const type_settings = {
  error: {
    styles: "bg-red-500",
    icon: "IconCheck",
  },
  warning: {
    styles: "bg-yellow-500",
    icon: "IconCheck",
  },
  success: {
    styles: "bg-green-500",
    icon: "IconCheck",
  },
};

function addMessage(message: Message): void {
  if (message.key && idxOfKey(message.key) > -1) closeKey(message.key);

  messages.push(message);
  if (message.timeout)
    setTimeout(() => {
      closeMessage(message.key);
    }, message.timeout);
}

function idxOfKey(key: string): number {
  return messages.findIndex((message) => {
    return message.key == key;
  });
}

function closeMessage(key: string): void {
  var idx = idxOfKey(key);
  var message = messages[idx];
  closeIdx(idx);
  if (message && message.isReloader) window.location.href = location.href;
}

function closeKey(key: string): void {
  if (typeof key == "undefined") return;
  closeIdx(idxOfKey(key));
}

function closeIdx(idx: number): void {
  if (idx > -1) messages.splice(idx, 1);
}
</script>
