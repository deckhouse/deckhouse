import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import ErrorPage from "@/pages/ErrorPage.vue";

export default {
  title: "Deckhouse UI/Pages/Other/Error",
  component: ErrorPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args) => ({
  components: { ErrorPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <ErrorPage v-bind="args" />
    </BaseLayout>
  `,
});

export const Page404 = Template.bind({});
Page404.args = {
  code: 404,
  text: "Страница не найдена.",
};

export const Page500 = Template.bind({});
Page500.args = {
  code: 500,
  text: "Что-то пошло не так. Перезагрузите страницу.",
};
