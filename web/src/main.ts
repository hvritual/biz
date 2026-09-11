import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./app/router";
import "./styles/tokens.css";
import "./styles/base.css";
if (import.meta.env.VITE_DATA_MODE && import.meta.env.VITE_DATA_MODE !== "preview") {
  throw new Error(
    "当前交付只包含 preview 数据适配器，未配置真实 API。禁止回退到演示数据。",
  );
}
const app = createApp(App);
app.use(createPinia()).use(router).mount("#app");
