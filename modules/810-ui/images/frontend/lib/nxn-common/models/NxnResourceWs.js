// import GlobalNxnFlash from 'nxn-common/services/GlobalNxnFlash.js';
// import CurrentEditSession from 'nxn-common/services/CurrentEditSession.js';
import NxnResourceHttp from "./NxnResourceHttp.js";
import { createConsumer } from "@rails/actioncable";
import { reactive } from "vue";

// TODO:
class NxnResourceCables {}
NxnResourceCables.cables = {};

class NxnResourceWs extends NxnResourceHttp {
  static initSubscription(channelName, params = {}) {
    this.storage = reactive({});
    this.cable = undefined;
    this.unappliedUpdates = {};
    this.channelName = channelName;
    this.channelParams = params;
    // TODO: put called actions in queue, TODO: do smth with unsubscribe?!
    if (!this.channel) this.channel = undefined;
  }

  // WARNING: requires initialized storage
  static subscribe(kwargs) {
    if (!this.getCable()) return;

    // params:, received:, handleConnected:
    if (!kwargs) kwargs = {};

    var channelName = kwargs.channelName || this.channelName;

    var channelIdentifier = Object.assign({ channel: channelName }, this.channelParams || {});
    console.log(`${this.klassName}.subscribe to ${JSON.stringify(channelIdentifier)} (${JSON.stringify(kwargs.params)})`);

    var klass = this;
    var channel;
    channel = this.getCable().subscriptions.create(Object.assign(channelIdentifier, kwargs.params || {}), {
      disconnected() {
        var errorText = `${channelName} subscription has been terminated by the server`;
        console.error(errorText);
        if (klass.onWsDisconnect) klass.onWsDisconnect(channel);
        //GlobalNxnFlash.proposeReload(errorText);
        console.log("GlobalNxnFlash.proposeReload(errorText);");
        return;
      },
      received(msg) {
        console.log("RECEIVED IN NXN", msg.message_type);
        if (!!kwargs && !!kwargs.received) return kwargs.received.call(klass, msg);

        // channel will be checked in `shouldIgnoreCallback`
        switch (msg.message_type) {
          case "__modelfeed-started":
            //GlobalNxnFlash.proposeReload('Sorry, but currently the only way for client to catch up with server after changefeed error is full reload.');
            console.log(
              "GlobalNxnFlash.proposeReload('Sorry, but currently the only way for client to catch up with server after changefeed error is full reload.');"
            );
            break;
          case "create":
            var item = klass.saveServerRepresentation(msg.message, {
              channel: channel,
            });
            klass.runChannelCallbacks("pure_channel_create", [item], {
              channel: channel,
            });
            break;
          case "update":
            console.log("GOT UPDATE!", msg.message);
            klass.saveServerRepresentation(msg.message, {
              channel: channel,
              toUnappliedUpdatesIfNotStored: klass.useUnappliedUpdates,
            });
            break;
          case "inconsequential_update":
            klass.saveInconsequentialUpdate(msg.message, {
              channel: channel,
              toUnappliedUpdatesIfNotStored: klass.useUnappliedUpdates,
            });
            break;
          case "delete":
            klass.nxndbDestroy(klass.toPrimaryKey(msg.message), {
              channel: channel,
              messageData: msg.message,
            });
            klass.runChannelCallbacks("pure_channel_delete", [klass.toPrimaryKey(msg.message)], { channel: channel });
            break;
          default:
            klass.runChannelCallbacks(msg.message_type, [msg.message], {
              channel: channel,
            });
        }
        return;
      },
      connected() {
        if (kwargs.connected) kwargs.connected.call(klass, channel);
      },
    });
    if (kwargs.klassChannel) this.channel = channel;
    return channel;
  }

  static unsubscribe() {
    if (this.channel) {
      if (this.flushQueryCache) this.flushQueryCache(this.channel);
      this.channel.unsubscribe();
      this.channel = undefined;
      this.unappliedUpdates = {};
    }
    return true;
  }

  // private

  static getCable(forceCablePath) {
    var cablePath = forceCablePath || this.cableUrl;
    if (!cablePath) return;

    if (!NxnResourceCables.cables[cablePath]) {
      NxnResourceCables.cables[cablePath] = this.createCable(cablePath);
    }
    return NxnResourceCables.cables[cablePath];
  }

  static createCable(cablePath) {
    var url = "";
    if (cablePath) {
      url = `${window.location.protocol == "http:" ? "ws" : "wss"}://${cablePath}`;
    }
    console.log("Cable URL:", url);
    // return createConsumer(`${url}/cable?edit_session_id=${'CurrentEditSession.id'}`);
    return createConsumer(url);
  }
}

export default NxnResourceWs;
