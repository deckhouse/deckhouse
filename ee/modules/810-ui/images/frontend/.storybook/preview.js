import { app } from '@storybook/vue3';
import initApp from "@/init";

await initApp({ app, initWS: false, initMocks: true });

export const parameters = {
  actions: { argTypesRegex: "^on[A-Z].*" },
  controls: {
    matchers: {
      color: /(background|color)$/i,
      date: /Date$/,
    },
  },
}
