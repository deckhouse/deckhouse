import { Server, WebSocket } from "mock-socket";
// @ts-ignore
import * as ActionCable from "@rails/actioncable";
// @ts-ignore
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";
import deckhouseConfig from "./objects/deckhouse_settings.json";
import deckhouseReleases from "./objects/deckhouse_releases.json";

const cloudProvider = import.meta.env.VITE_CLOUD_PROVIDER || "aws";
const { default: instanceclasses } = await import(`./objects/${cloudProvider}instanceclasses.json`);

// replace adapter to mocked
ActionCable.adapters.WebSocket = WebSocket;
ActionCable.logger.enabled = true;

const wsURL = `ws://${NxnResourceWs.cableUrl}`;
export const mockServer = new Server(wsURL);

function log(msg: string, ...args: any[]): void {
  console.log(`WSMock: ${msg}`, ...args);
}

function arraySample(array: any[]): any {
  return array[Math.floor(Math.random() * array.length)];
}

function randomString(): string {
  return (Math.random() + 1).toString(36).substring(7);
}

mockServer.on("connection", (socket) => {
  function cableSend(data: any, identifier: any = null): void {
    if (typeof identifier === "object") {
      data.identifier = JSON.stringify(identifier);
    } else if (typeof identifier === "string") {
      data.identifier = identifier;
    }

    if (data.type != "ping") {
      log("send msg", data);
    }
    socket.send(JSON.stringify(data));
  }

  log("CONNECTED");

  socket.on("message", (msg: any) => {
    log("MESSAGE FROM WS", msg);
    msg = JSON.parse(msg);
    switch (msg.command) {
      case "subscribe": {
        cableSend({ type: "confirm_subscription" }, msg.identifier);
        break;
      }
    }
  });
  socket.on("close", () => {
    log("CLOSE FROM WS");
  });

  socket.on("error", (err) => {
    log("ERROR FROM WS", err);
  });

  // ActionCable PINGER
  setInterval(() => {
    cableSend({ type: "ping" });
  }, 3000);

  // ActionCable some periodic data messages

  //// DeckhouseConfig
  function deckhouseConfigPulse() {
    deckhouseConfig.spec.settings.release.mode = arraySample(["Auto", "Manual"]);
    deckhouseConfig.spec.settings.releaseChannel = arraySample(["Stable", "Alpha", "Beta"]);
    cableSend(
      {
        message: {
          message_type: "update",
          message: deckhouseConfig,
        },
      },
      { channel: "GroupResourceChannel", groupResource: "moduleconfigs.deckhouse.io" }
    );
  }

  setInterval(deckhouseConfigPulse, 7000);

  //// DeckhouseReleases

  function deckhouseReleasesPulse() {
    const releases = [...deckhouseReleases];

    const randomRelease = arraySample(releases);
    const type = arraySample(["create", "update", "delete"]);

    switch (type) {
      case "create": {
        randomRelease.metadata.name = randomString();
        randomRelease.metadata.uid = randomString();
        break;
      }
      case "update": {
        randomRelease.metadata.name = randomString();
      }
    }

    cableSend(
      {
        message: {
          message_type: type,
          message: randomRelease,
        },
      },
      { channel: "GroupResourceChannel", groupResource: "deckhousereleases.deckhouse.io" }
    );
  }

  setInterval(deckhouseReleasesPulse, 5000);

  //// Instance Classes

  function instanceClassesPulse() {
    const ics = [...instanceclasses];

    const randomIc = arraySample(ics);
    const type = arraySample(["create", "update", "delete"]);

    switch (type) {
      case "create": {
        randomIc.metadata.name = randomString();
        randomIc.metadata.uid = randomString();
        break;
      }
      case "update": {
        randomIc.metadata.name = randomString();
        break;
      }
    }

    cableSend(
      {
        message: {
          message_type: type,
          message: randomIc,
        },
      },
      { channel: "GroupResourceChannel", groupResource: `${cloudProvider}instanceclasses.deckhouse.io` }
    );
  }

  setInterval(instanceClassesPulse, 5000);
});
