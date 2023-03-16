import { createRouter, createWebHistory } from "vue-router";
import ReleasesPage from "../pages/ReleasesPage.vue";
import DeckhouseSettingsPage from "../pages/DeckhouseSettingsPage.vue";
import NodeGroupListPage from "../pages/NodeGroupListPage.vue";
import NodeListPage from "../pages/NodeListPage.vue";
import NodePage from "@/pages/NodePage.vue";

export const routes = [
  {
    path: "/",
    name: "home",
    component: ReleasesPage,
  },
  {
    path: "/settings",
    name: "DeckhouseSettings",
    component: DeckhouseSettingsPage,
  },
  {
    path: "/node_group_list",
    name: "NodeGroupList",
    component: NodeGroupListPage,
  },
  {
    path: "/node_groups/:ng_name/nodes",
    name: "NodeList",
    component: NodeListPage,
  },
  {
    path: "/node_groups/:ng_name/nodes/:name",
    name: "NodeShow",
    component: NodePage,
  },
  {
    path: "/node_groups/:ng_name/nodes/:name/edit",
    name: "NodeEdit",
    component: NodePage,
  },
];

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
});

export default router;
