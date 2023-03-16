import type { Meta, Story } from "@storybook/vue3";

import SidebarComponent from "@/components/sidebar/TheSidebar.vue";

export default {
  title: "Deckhouse UI/Components/Sidebar",
  component: SidebarComponent,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args) => ({
  components: { SidebarComponent },
  setup() {
    return { args };
  },
  template: '<SidebarComponent v-bind="args" />',
});

export const Default = Template.bind({});
