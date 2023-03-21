import { rest } from "msw";

import discoveries from "./objects/discovery.json";
import deckhouseConfig from "./objects/deckhouse_settings.json";
import deckhouseReleases from "./objects/deckhouse_releases.json";
import nodes from "./objects/nodes.json";
import nodeGroups from "./objects/nodegroups.json";
import awsMachineClasses from "./objects/awsinstanceclasses.json";
import openstackMachineClasses from "./objects/openstackinstanceclasses.json";

// @ts-ignore
import NxnResourceHttp from "@lib/nxn-common/models/NxnResourceHttp";

const discovery = discoveries[(import.meta.env.VITE_CLOUD_PROVIDER || "aws") as keyof typeof discoveries];

console.log("import.meta.env.VITE_CLOUD_PROVIDER", import.meta.env.VITE_CLOUD_PROVIDER, discovery);

function getDeckhouseRelease(name: any): any {
  return deckhouseReleases.find((dr) => dr.metadata.name == name);
}

function getNodeGroup(name: any): any {
  return nodeGroups.find((dr) => dr.metadata.name == name);
}

function getNodesByGroup(group: string | null): any[] {
  // KOSTYL: hack to get all nodes
  if (group && group != "all") {
    return nodes.filter((n) => n.metadata.labels["node.deckhouse.io/group"] == group);
  }

  return nodes;
}
function getNode(name: any): any {
  return nodes.find((n) => n.metadata.name == name);
}

function getAwsMachineClass(name: any): any {
  return awsMachineClasses.find((n) => n.metadata.name == name);
}

console.log("HELLO!", NxnResourceHttp.apiUrl("discovery"));

export const handlers = {
  discovery: [
    rest.get(NxnResourceHttp.apiUrl("discovery"), (req, res, ctx) => {
      return res(ctx.json(discovery));
    }),
  ],

  deckhouseConfig: [
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/moduleconfigs/deckhouse"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(deckhouseConfig));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/moduleconfigs/deckhouse"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(req.json()));
    }),
  ],

  releases: [
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/deckhousereleases"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(deckhouseReleases));
    }),
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/deckhousereleases/:name"), (req, res, ctx) => {
      return res(ctx.json(getDeckhouseRelease(req.params.name)));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/deckhousereleases/:name"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(req.json()));
    }),
  ],

  nodes: [
    rest.get(NxnResourceHttp.apiUrl("k8s/nodes"), (req, res, ctx) => {
      const group = req.url.searchParams.get("node.deckhouse.io/group");
      return res(ctx.delay(500), ctx.json(getNodesByGroup(group)));
    }),
    rest.get(NxnResourceHttp.apiUrl("k8s/nodes/:name"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(getNode(req.params.name)));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/nodes/:name"), async (req, res, ctx) => {
      const json = await req.json();
      console.log("REQ!", json);

      return res(ctx.delay(500), ctx.json(json));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/nodes/:name/drain"), (req, res, ctx) => {
      return res(ctx.delay(2000), ctx.status(204));
    }),
  ],

  nodeGroups: [
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/nodegroups"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(nodeGroups));
    }),

    rest.post(NxnResourceHttp.apiUrl("k8s/deckhouse.io/nodegroups"), async (req, res, ctx) => {
      const json = await req.json();
      return res(ctx.delay(500), ctx.json(json));
    }),

    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/nodegroups/:name"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(getNodeGroup(req.params.name)));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/nodegroups/:name"), async (req, res, ctx) => {
      const json = await req.json();
      return res(ctx.delay(500), ctx.json(json));
    }),
    rest.delete(NxnResourceHttp.apiUrl("k8s/deckhouse.io/nodegroups/:name"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.status(200));
    }),
  ],

  // Instanceclasses
  awsInstanceClasses: [
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/awsinstanceclasses"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(awsMachineClasses));
    }),

    rest.post(NxnResourceHttp.apiUrl("k8s/deckhouse.io/awsinstanceclasses"), async (req, res, ctx) => {
      const json = await req.json();
      return res(ctx.delay(500), ctx.json(json));
    }),

    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/awsinstanceclasses/:name"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(getAwsMachineClass(req.params.name)));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/awsinstanceclasses/:name"), async (req, res, ctx) => {
      const json = await req.json();
      return res(ctx.delay(500), ctx.json(json));
    }),
    rest.delete(NxnResourceHttp.apiUrl("k8s/deckhouse.io/awsinstanceclasses/:name"), (req, res, ctx) => {
      return res(ctx.delay(1500), ctx.status(200));
    }),
  ],

  //  Openstack
  openstackInstanceClasses: [
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/openstackinstanceclasses"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(openstackMachineClasses));
    }),
    rest.post(NxnResourceHttp.apiUrl("k8s/deckhouse.io/openstackinstanceclasses"), async (req, res, ctx) => {
      const json = await req.json();
      return res(ctx.delay(500), ctx.json(json));
    }),
    rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/openstackinstanceclasses/:name"), (req, res, ctx) => {
      return res(ctx.delay(500), ctx.json(getAwsMachineClass(req.params.name)));
    }),
    rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/openstackinstanceclasses/:name"), async (req, res, ctx) => {
      const json = await req.json();
      return res(ctx.delay(500), ctx.json(json));
    }),
    rest.delete(NxnResourceHttp.apiUrl("k8s/deckhouse.io/openstackinstanceclasses/:name"), (req, res, ctx) => {
      return res(ctx.delay(1500), ctx.status(200));
    }),
  ],
};

export const rawHandlers = Object.values(handlers)
  .filter(Boolean)
  .reduce((handlers, handlersList) => handlers.concat(handlersList), []);
