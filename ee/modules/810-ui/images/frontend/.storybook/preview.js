import { app } from '@storybook/vue3';
import initApp from "@/init";
import { mswDecorator } from 'msw-storybook-addon';

await initApp({ app, initWS: false, initMocks: "storybook" });

const { handlers } = await import("@/mocks/handlers");

export const parameters = {
  actions: { argTypesRegex: "^on[A-Z].*" },
  controls: {
    matchers: {
      color: /(background|color)$/i,
      date: /Date$/,
    },
  },
  msw: {
    handlers
  }
}

export const decorators = [ mswDecorator ]
