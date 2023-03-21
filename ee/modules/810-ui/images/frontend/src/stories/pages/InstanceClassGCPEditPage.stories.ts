import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import InstanceClassGCPEditPage from "@/pages/instanceclass/InstanceClassGCPEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/GCP Edit",
  component: InstanceClassGCPEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassGCPEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassGCPEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
