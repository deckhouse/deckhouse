<template>
  <div class="flex flex-col w-full items-start justify-between mb-6">
    <div v-if="$slots.tabs" :class="$slots.filter || $slots.actions ? 'mb-3' : ''">
      <slot name="tabs" />
    </div>
    <div class="flex w-full items-end justify-between">
      <div v-if="$slots.filter">
        <slot name="filter" />
      </div>
      <div v-if="$slots.actions">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>
