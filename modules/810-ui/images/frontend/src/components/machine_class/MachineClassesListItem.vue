<template>
  <CardBlock
    title="The-biggest-node-group-name-imaginable-is-63-symbols-max-woohoo"
    url="https://flant.ru/"
    notice-type="warning"
    :badges="[{ id: 1, title: 'STOP', type: 'warning' }]"
    :icon="'IconAWSLogo'"
  >
    <template #content>
      <CardParamGrid>
        <CardParam title="Тип VM" value="m5.xlarge" />
        <CardParam title="ЦПУ" value="4" />
        <CardParam title="Память" value="16 Gib" />
        <CardParam title="Диск" value="20 G gp3" />
        <CardParam title="Доп группы безопасности" :value="['frontend', 'frontend']" />
        <CardParam title="Дополнительные теги" :value="['team: sysadmin', 'tag: subject']" />
      </CardParamGrid>
    </template>
    <template #actions>
      <ButtonBlock :icon="'IconCancel'" title="Удалить" type="primary-subtle"></ButtonBlock>
    </template>
    <template #notice>
      Scary error message! You have been warned! Scary error message! You have been warned! Scary error message! You have been warned! Scary
      error message! You have been warned
    </template>
  </CardBlock>
</template>

<script setup lang="ts">
import ButtonBlock from "../common/button/ButtonBlock.vue";

import CardBlock from "../common/card/CardBlock.vue";
import CardParam from "../common/card/CardParam.vue";
import CardParamGrid from "../common/card/CardParamGrid.vue";
</script>
