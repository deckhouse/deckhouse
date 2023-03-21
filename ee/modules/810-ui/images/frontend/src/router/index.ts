import { createRouter, createWebHistory, type RouteLocationNormalizedLoaded } from "vue-router";
import ReleasesPage from "../pages/ReleasesPage.vue";
import DeckhouseSettingsPage from "../pages/DeckhouseSettingsPage.vue";
import NodeGroupListPage from "../pages/NodeGroupListPage.vue";
import NodeGroupPage from "@/pages/NodeGroupPage.vue";
import NodeListPage from "../pages/NodeListPage.vue";
import NodePage from "@/pages/NodePage.vue";
import InstanceClassesListPage from "@/pages/InstanceClassesListPage.vue";
import InstanceClassPage from "@/pages/InstanceClassPage.vue";

import type { MenuItem } from "primevue/menuitem";

export const routes = [
  {
    path: "/",
    name: "Home",
    component: ReleasesPage,
    meta: {
      breadcrumbs(): Array<MenuItem> {
        return [{ label: "Обновления", to: { name: "Home" }, active: true }];
      },
    },
  },
  {
    path: "/settings",
    name: "DeckhouseSettings",
    component: DeckhouseSettingsPage,
    meta: {
      breadcrumbs(_route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        // TODO: link parent
        return [{ label: "Обновления", to: { name: "Home" }, active: true }];
      },
    },
  },
  {
    path: "/nodegroups",
    name: "NodeGroupList",
    component: NodeGroupListPage,
    meta: {
      breadcrumbs(_route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [{ label: "Группы узлов", to: { name: "NodeGroupList" }, active: true }];
      },
    },
  },
  {
    path: "/nodegroups/new", //TODO: conflict with name=`new` ?
    name: "NodeGroupNew",
    component: NodeGroupPage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: `Добавление новой группы типа ${route.params.type}`, active: true },
        ];
      },
    },
  },
  {
    path: "/nodegroups/:name",
    name: "NodeGroupShow",
    component: NodeGroupPage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        // TODO: link parent
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: route.params.name.toString(), to: { name: "NodeGroupList", params: { name: route.params.name } }, active: true },
        ];
      },
    },
  },
  {
    path: "/nodegroups/:name/edit",
    name: "NodeGroupEdit",
    component: NodeGroupPage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: route.params.name.toString(), to: { name: "NodeGroupShow", params: { name: route.params.name } }, active: true },
        ];
      },
    },
  },
  {
    path: "/nodes",
    name: "NodeListAll",
    component: NodeListPage,
    meta: {
      breadcrumbs(_route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: "Узлы всех групп", to: { name: "NodeListAll" }, active: true },
        ];
      },
    },
  },
  {
    path: "/nodegroups/:ng_name/nodes",
    name: "NodeList",
    component: NodeListPage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: route.params.ng_name.toString(), to: { name: "NodeGroupShow", params: { name: route.params.ng_name } }, active: true },
        ];
      },
    },
  },
  {
    path: "/nodegroups/:ng_name/nodes/:name",
    name: "NodeShow",
    component: NodePage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: route.params.ng_name.toString(), to: { name: "NodeGroupShow", params: { name: route.params.ng_name } } },
          { label: "Список узлов", to: { name: "NodeList", params: { ng_name: route.params.ng_name } } },
          {
            label: route.params.name.toString(),
            to: { name: "NodeShow", params: { ng_name: route.params.ng_name, name: route.params.name } },
            active: true,
          },
        ];
      },
    },
  },
  {
    path: "/nodegroups/:ng_name/nodes/:name/edit",
    name: "NodeEdit",
    component: NodePage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Группы узлов", to: { name: "NodeGroupList" } },
          { label: route.params.ng_name.toString(), to: { name: "NodeGroupShow", params: { name: route.params.ng_name } } },
          { label: "Список узлов", to: { name: "NodeList", params: { ng_name: route.params.ng_name } } },
          {
            label: route.params.name.toString(),
            to: { name: "NodeShow", params: { ng_name: route.params.ng_name, name: route.params.name } },
            active: true,
          },
        ];
      },
    },
  },
  {
    path: "/instanceclasses",
    name: "InstanceClassesList",
    component: InstanceClassesListPage,
    meta: {
      breadcrumbs(_route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [{ label: "Классы машин", icon: "pi pi-shopping-bag", to: { name: "InstanceClassesList" }, active: true }];
      },
    },
  },
  {
    path: "/instanceclasses/:name",
    name: "InstanceClassShow",
    component: InstanceClassPage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Классы машин", icon: "pi pi-shopping-bag", to: { name: "InstanceClassesList" } },
          { label: route.params.name.toString(), to: { name: "InstanceClassShow", params: { name: route.params.name } }, active: true },
        ];
      },
    },
  },
  {
    path: "/instanceclasses/new", // TODO: conflict with IC named `new` ?
    name: "InstanceClassNew",
    component: InstanceClassPage,
    meta: {
      breadcrumbs(_route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Классы машин", icon: "pi pi-shopping-bag", to: { name: "InstanceClassesList" } },
          { label: "Новый", to: { name: "InstanceClassNew" } },
        ];
      },
    },
  },
  {
    path: "/instanceclasses/:name/edit",
    name: "InstanceClassEdit",
    component: InstanceClassPage,
    meta: {
      breadcrumbs(route: RouteLocationNormalizedLoaded): Array<MenuItem> {
        return [
          { label: "Классы машин", icon: "pi pi-shopping-bag", to: { name: "InstanceClassesList" } },
          { label: route.params.name.toString(), to: { name: "InstanceClassShow", params: { name: route.params.name } }, active: true },
        ];
      },
    },
  },
];

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
});

export default router;
