<template>
  <div class="flex flex-col">
    <div class="-m-1.5 overflow-x-auto">
      <div class="p-1.5 min-w-full inline-block align-middle">
        <div class="overflow-hidden">
          <table class="min-w-full">
            <thead class="border-b-2 border-slate-100">
              <tr>
                <th v-for="field in fields" :key="field" scope="col" 
                  class="pr-6 py-2 text-left text-slate-800 uppercase">
                    {{ field }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, index) in data" :key="item">
                <td v-for="field in item" :key="field" 
                  class="pr-6 py-2 whitespace-nowrap text-sm text-slate-500">
                  {{ field }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  fields: Array,
  data: Array
});
</script>
