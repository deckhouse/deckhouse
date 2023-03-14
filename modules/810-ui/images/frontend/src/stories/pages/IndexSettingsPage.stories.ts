import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import IndexSettingsPage from "../../pages/IndexSettingsPage.vue";

export default {
  title: "Deckhouse UI/Pages/IndexSettings",
  component: IndexSettingsPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { IndexSettingsPage, BaseLayout },

  setup() {
    return { args, releases: releases };
  },

  template: `
    <BaseLayout>
      <IndexSettingsPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});