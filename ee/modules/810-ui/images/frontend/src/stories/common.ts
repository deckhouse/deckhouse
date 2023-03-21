import { app } from "@storybook/vue3";
import { makeDecorator } from "@storybook/addons";

export const routerDecorator = makeDecorator({
  name: "routerDecorator",
  parameterName: "router",
  wrapper: (storyFn, context, { parameters }) => {
    const router = app.config.globalProperties.$router;
    console.log("HAHA", parameters, router.getRoutes());

    if (!parameters) return storyFn(context);

    router.replace(parameters.currentRoute);
    console.log("router", router, router.currentRoute);

    return storyFn(context);
  },
});
