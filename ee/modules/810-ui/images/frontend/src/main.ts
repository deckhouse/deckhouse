import { createApp } from "vue";
import App from "./App.vue";
import initApp from "./init";
import Discovery from "./models/Discovery";

const app = createApp(App);
initApp({ app, initMocks: import.meta.env.DEV && !import.meta.env.VITE_NO_MOCK }).then(() => {
  Discovery.load().then(() => {
    app.mount("#app");
  });
});
