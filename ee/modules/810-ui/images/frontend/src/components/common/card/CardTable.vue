<template>
  <div class="flex flex-col">
    <div class="-m-1.5 overflow-x-auto">
      <div class="p-1.5 min-w-full inline-block align-middle">
        <div class="overflow-hidden">
          <table class="min-w-full">
            <thead class="bg-gray-50">
              <tr>
                <th v-for="field in fields" :key="field" scope="col" 
                  class="px-6 py-3 text-left font-medium text-slate-800 bg-blue-50">
                    {{ field }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, index) in data" :key="item" :class="index%2 != 0 ? 'bg-slate-100' : ''">
                <td v-for="field in item" :key="field" 
                  class="px-6 py-4 whitespace-nowrap text-sm text-slate-500">
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
