import type { RouteParamsRaw } from "vue-router";
import * as Icons from "../common/icon";
import { UpdateWindowsDays } from "./consts";

export type IconsType = keyof typeof Icons;

export type ISidebarItem = {
  icon: IconsType;
  title: string;
  routeNames: string[];
  routeParams?: {
    [key: string]: any;
  };
  children?: ISidebarItem[];
  badge?: Ref<number | boolean>;
};

export type TabsItem = {
  title: string;
  active?: boolean;
  badge?: Ref<number>;
  disabled?: boolean;
  routeName: string;
  routeParams?: RouteParamsRaw;
};

export type Badge = {
  title?: string;
  type: "success" | "warning" | "info" | "error";
  loading?: boolean;
  styles?: string;
};

export type IStatusCondition = {
  type: "Ready" | "Updating" | "WaitingForDisruptiveApproval" | "Error" | "Scaling";
  status: "True" | "False";
  message?: string;
};

export type IKeyValue = {
  key: string;
  value: string;
};

export type ITaint = IKeyValue & {
  effect: string;
};

export type IUpdateWindowDate = typeof UpdateWindowsDays;

export interface IUpdateWindow {
  days: IUpdateWindowDate[];
  from: string;
  to: string;
}
