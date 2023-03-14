export type ISidebarItem = {
  id: string;
  title: string;
  active: boolean;
  routeNames: Array<string>;
};

export type ITabsItem = {
  id: string;
  title: string;
  active?: boolean;
  badge?: Ref<number>;
  routeName: string;
};

export type IBadge = {
  id: number | string;
  title?: string;
  type: "success" | "warning";
  styles?: string;
};
