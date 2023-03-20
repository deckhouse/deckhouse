import type { App } from "vue";
import { ref } from 'vue'

import router from "./router";

import "preline";

// @ts-ignore
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";
// @ts-ignore
import NxnResourceHttp from "@lib/nxn-common/models/NxnResourceHttp";

import PrimeVue from "primevue/config";
import VueTippy from "vue-tippy";

import dayjs from "dayjs";
import utc from "dayjs/plugin/utc";
import timezone from "dayjs/plugin/timezone";
import customParseFormat from "dayjs/plugin/customParseFormat";
import advancedFormat from "dayjs/plugin/advancedFormat";
import "dayjs/locale/ru"; // import locale

import "./index.css";
import "./assets/main.css";

import "primevue/resources/themes/lara-light-blue/theme.css";
import "primevue/resources/primevue.min.css";
import "primeicons/primeicons.css";

import "tippy.js/dist/tippy.css";

export default async function initApp({
  app,
  initWS = true,
  initMocks = false,
  initRouter = true,
}: {
  app: App;
  initWS?: boolean;
  initMocks?: boolean;
  initRouter?: boolean;
}): Promise<void> {
  if (initRouter) app.use(router);

  app.use(PrimeVue);
  app.use(VueTippy);

  dayjs.extend(utc);
  dayjs.extend(timezone);
  dayjs.extend(customParseFormat);
  dayjs.extend(advancedFormat);
  dayjs.locale("ru");

  app.provide("globalSettings", ref({
    "specMode": true
  }))

  NxnResourceHttp.baseUrl = import.meta.env.VITE_API_BASE_URL || "/api";
  initWS = initWS && !import.meta.env.VITE_NO_WS;
  if (initWS) {
    NxnResourceWs.cableUrl = import.meta.env.VITE_WS_URL || `${window.location.host}/api/subscribe`;
  }

  if (initMocks) {
    const { worker } = await import("./mocks/browser");
    worker.start({
      onUnhandledRequest: "bypass",
    });

    // This initializes MockServer and mocks window.WebSocket and ActionCable.adapter.WebSockets
    if (initWS) {
      await import("./mocks/actioncable");
    }
  }
}
