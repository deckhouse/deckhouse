import type { Meta, Story } from "@storybook/vue3";

import DeckhouseModuleSettings from "@/components/releases/DeckhouseModuleSettings.vue";

export default {
  title: "Deckhouse UI/Components/DeckhouseModuleSettings",
  component: DeckhouseModuleSettings,
  parameters: { layout: "fullscreen" },
} as Meta;


const Template: Story = (args) => ({
  components: { DeckhouseModuleSettings },
  setup() {
    return { args };
  },
  template: '<DeckhouseModuleSettings v-bind="args" />',
});

export const Default = Template.bind({});
