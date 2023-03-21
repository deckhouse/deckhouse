import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import InstanceClassAzureEditPage from "@/pages/instanceclass/InstanceClassAzureEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/Azure Edit",
  component: InstanceClassAzureEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassAzureEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassAzureEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
