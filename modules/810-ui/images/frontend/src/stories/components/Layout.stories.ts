import type { Meta, Story } from "@storybook/vue3";
import BaseLayoutComponent from "@/components/layout/BaseLayout.vue";

import * as SidebarStories from "./Sidebar.stories";

export default {
  title: "Deckhouse UI/Components/Layout",
  component: BaseLayoutComponent,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args) => ({
  components: { BaseLayoutComponent },
  setup() {
    return { args };
  },
  template: '<BaseLayoutComponent v-bind="args" />',
});

export const Base = Template.bind({});
