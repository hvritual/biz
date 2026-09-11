import { ref } from "vue";
import { defineStore } from "pinia";
export const useToast = defineStore("toast", () => {
  const message = ref("");
  let timer: ReturnType<typeof setTimeout> | undefined;
  function show(text: string) {
    message.value = text;
    clearTimeout(timer);
    timer = setTimeout(() => {
      message.value = "";
    }, 5000);
  }
  return { message, show };
});
