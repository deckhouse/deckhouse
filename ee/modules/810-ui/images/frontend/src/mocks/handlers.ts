import { rest } from "msw";

import discovery from "./objects/discovery.json";
import deckhouseConfig from "./objects/deckhouse_settings.json";
import deckhouseReleases from "./objects/deckhouse_releases.json";
import nodes from "./objects/nodes.json";

// @ts-ignore
import NxnResourceHttp from "@lib/nxn-common/models/NxnResourceHttp";

function getDeckhouseRelease(name: any): any {
  return deckhouseReleases.find((dr) => dr.metadata.name == name);
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

export const handlers = [
  // Discovery
  rest.get(NxnResourceHttp.apiUrl("discovery"), (req, res, ctx) => {
    return res(ctx.json(discovery));
  }),

  // Deckhouse Config
  rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/moduleconfigs/deckhouse"), (req, res, ctx) => {
    return res(ctx.delay(500), ctx.json(deckhouseConfig));
  }),
  rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/moduleconfigs/deckhouse"), (req, res, ctx) => {
    return res(ctx.delay(500), ctx.json(req.json()));
  }),

  // Releases
  rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/deckhousereleases"), (req, res, ctx) => {
    return res(ctx.delay(500), ctx.json(deckhouseReleases));
  }),
  rest.get(NxnResourceHttp.apiUrl("k8s/deckhouse.io/deckhousereleases/:name"), (req, res, ctx) => {
    return res(ctx.json(getDeckhouseRelease(req.params.name)));
  }),
  rest.put(NxnResourceHttp.apiUrl("k8s/deckhouse.io/deckhousereleases/:name"), (req, res, ctx) => {
    return res(ctx.delay(500), ctx.json(req.json()));
  }),

  // Nodes
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
];
