import type { FormMeta } from "vee-validate";
import type { ComputedRef } from "vue";
import { onBeforeRouteLeave } from "vue-router";

interface useFormLeaveGuard {
  formMeta: ComputedRef<FormMeta<any>>;
  onLeave(): void;
}

export default function useFormLeaveGuard({ formMeta, onLeave }: useFormLeaveGuard) {
  onBeforeRouteLeave((to, from) => {
    if (!formMeta.value.dirty) return true;

    if (confirm("Вы уходите? А как же ваши изменения?")) {
      onLeave();
      return true;
    } else {
      return false;
    }
  });
}
