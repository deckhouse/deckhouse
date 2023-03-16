import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import ReleasesPage from "@/pages/ReleasesPage.vue";

import DeckhouseRelease from "@/models/DeckhouseRelease";

export default {
  title: "Deckhouse UI/Pages/Releases",
  component: ReleasesPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { ReleasesPage, BaseLayout },

  setup() {
    return { args, releases: releases };
  },

  template: `
    <BaseLayout>
      <ReleasesPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
Default.loaders = [
  async () => ({
    releases: await DeckhouseRelease.query(),
  }),
];
