import { ref, watch, type Ref } from "vue";

interface useSpecMode {
  specMode: Ref<boolean>;
}

const loadState = (default_: boolean = true): boolean => {
  return typeof localStorage.specMode === "undefined" ? default_ : localStorage.specMode == "true";
};

const saveState = (): void => {
  localStorage.specMode = String(specMode.value);
};

const specMode = ref<boolean>(loadState());

watch(specMode, () => saveState());

export default function useSpecMode(): useSpecMode {
  return {
    specMode,
  };
}
