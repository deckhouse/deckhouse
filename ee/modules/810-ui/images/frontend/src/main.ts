import { createApp } from "vue";
import App from "./App.vue";
import initApp from "./init";

const app = createApp(App);
initApp({ app, initMocks: import.meta.env.DEV && !import.meta.env.VITE_NO_MOCK }).then(() =>{
  app.mount("#app");
});
