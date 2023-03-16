import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import IndexPage from "@/pages/IndexPage.vue";

import DeckhouseRelease from "@/models/DeckhouseRelease";

export default {
  title: "Deckhouse UI/Pages/Index",
  component: IndexPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { IndexPage, BaseLayout },

  setup() {
    return { args, releases: releases };
  },

  template: `
    <BaseLayout>
      <IndexPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
Default.loaders = [
  async () => ({
    releases: await DeckhouseRelease.query(),
  }),
];
