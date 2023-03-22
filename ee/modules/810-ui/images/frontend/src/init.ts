import type { App } from "vue";
import { ref } from "vue";

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

import { configure as veeValidateConfigure } from "vee-validate";
import { localize as veeValidateLocalize } from "@vee-validate/i18n";
import veeValidateRu from "@vee-validate/i18n/dist/locale/ru.json";

import "./index.css";
import "./assets/main.css";

import "primevue/resources/themes/lara-light-blue/theme.css";
import "primevue/resources/primevue.min.css";
import "primeicons/primeicons.css";

import "tippy.js/dist/tippy.css";
import Discovery from "./models/Discovery";

import * as ActionCable from "@rails/actioncable";
ActionCable.logger.enabled = true;

// TODO: kostyl???
async function waitWsConnection() {
  const cable = NxnResourceWs.getCable();

  if (!cable.connection.isOpen()) {
    cable.ensureActiveConnection();

    const timeout = 2000;
    const intrasleep = 50;
    const ttl = timeout / intrasleep; // time to loop
    let loop = 0;
    while (!cable.connection.isOpen() && loop < ttl) {
      ActionCable.logger.log("waiting for WS connection...");
      await new Promise((resolve) => setTimeout(resolve, intrasleep));
      loop++;
    }
  }
}

export default async function initApp({
  app,
  initWS = true,
  initMocks = false,
  initRouter = true,
}: {
  app: App;
  initWS?: boolean;
  initMocks?: "app" | "storybook" | false;
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

  // TODO: Why isn't it working?
  veeValidateConfigure({
    generateMessage: veeValidateLocalize({
      ru: veeValidateRu,
    }),
  });

  NxnResourceHttp.baseUrl = import.meta.env.VITE_API_BASE_URL || "/api";
  initWS = initWS && !import.meta.env.VITE_NO_WS;
  if (initWS) {
    NxnResourceWs.cableUrl = import.meta.env.VITE_WS_URL || `${window.location.host}/api/subscribe`;
  }

  if (initMocks == "app") {
    const { worker } = await import("./mocks/browser");
    worker.start({
      onUnhandledRequest: "bypass",
    });

    // This initializes MockServer and mocks window.WebSocket and ActionCable.adapter.WebSockets
    if (initWS) {
      await import("./mocks/actioncable");
    }
  } else if (initMocks == "storybook") {
    const { initialize, getWorker } = await import("msw-storybook-addon");
    const { handlers } = await import("@/mocks/handlers");

    initialize({
      onUnhandledRequest: "bypass",
    });

    getWorker().use(...handlers.discovery);
  }
  if (initWS) await waitWsConnection(); // TODO: KOSTYL??? wait ws connection before starting application. But this function is not async :/

  await Discovery.load();
}
