import { app } from "@storybook/vue3";
import { makeDecorator } from "@storybook/addons";

export const routerDecorator = makeDecorator({
  name: "routerDecorator",
  parameterName: "router",
  wrapper: (storyFn, context, { parameters }) => {
    console.log("HAHA", parameters);

    const router = app.config.globalProperties.$router;

    router.replace(parameters.currentRoute);
    console.log('router', router, router.currentRoute);

    return storyFn(context);
  },
});
