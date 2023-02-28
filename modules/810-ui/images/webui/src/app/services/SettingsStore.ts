import { MuteItems } from "../i18n"

type MuteOptions = Map<keyof MuteItems, boolean>

// from=unix-time&to=unix-time&step=300&mute=Acd!InfMnt&expand=synthetic!control-plane
export interface Settings {
  from: number
  to: number | string
  step: number
  fmt: string
  mute: MuteOptions
  expand: Map<string, boolean>
}

export class SettingsStore {
  public static defaultMuteFlags: MuteOptions = new Map([
    ["Acd", false],
    ["Mnt", true],
    ["InfAcd", true],
    ["InfMnt", true],
  ])

  load(): Settings {
    let hash = location.hash.substr(1)
    return this.settingsFromHash(hash)
  }

  save(settings: Settings) {
    let newHash = "#" + this.settingsToHash(settings)
    if (history.pushState) {
      history.pushState(null, null, newHash)
    } else {
      location.hash = newHash
    }
  }

  settingsFromHash(hash: string): Settings {
    let me = this
    let settings: Settings = {
      from: NaN,
      to: NaN,
      step: NaN,
      fmt: undefined,
      mute: undefined,
      expand: undefined,
    }
    hash.split("&").forEach(function (part) {
      let item = part.split("=")
      let k = item[0]
      let v: string = decodeURIComponent(item[1])
      if (k === "mute") {
        settings[k] = me.decodeMuteTypes(v)
      }
      if (v === "") {
        return
      }
      if (k === "expand") {
        settings[k] = me.decodeExpand(v)
      }
      if (k === "from") {
        settings[k] = +v
      }
      if (k === "to") {
        if (v === "now") {
          settings[k]
        } else {
          settings[k] = +v
        }
      }
      if (k === "step") {
        settings[k] = +v
      }
      if (k === "fmt") {
        settings[k] = v
      }
    })
    //console.log("Load settings from hash:", settings);
    return settings
  }

  settingsToHash(settings: Settings): string {
    let pairs = []
    let k: keyof Settings
    for (k in settings) {
      if (settings.hasOwnProperty(k)) {
        if (settings[k] === undefined) {
          continue
        }
        let v: string = ""
        if (k === "mute") {
          v = this.encodeMuteTypes(settings[k])
        } else if (k === "expand") {
          v = this.encodeExpand(settings[k])
          // do not save empty expand.
          if (v === "") {
            continue
          }
        } else {
          v = "" + settings[k]
        }
        pairs.push(encodeURIComponent(k) + "=" + encodeURIComponent(v))
      }
    }
    //console.log("Save settings to hash:", settings);
    return pairs.join("&")
  }

  decodeMuteTypes(input: string): MuteOptions {
    let res: MuteOptions = new Map()
    input = "!" + input + "!"
    for (let k of SettingsStore.defaultMuteFlags.keys()) {
      res.set(k, input.indexOf("!" + k + "!") > -1)
    }
    return res
  }

  encodeMuteTypes(mute: MuteOptions): string {
    let res: string[] = []
    for (let k of mute.keys()) {
      if (mute.get(k)) {
        res.push(k)
      }
    }
    return res.join("!")
  }

  decodeExpand(input: string): Map<string, boolean> {
    let res = new Map<string, boolean>()
    let parts = input.split("!")
    for (let i = 0; i < parts.length; i++) {
      if (parts[i] === "") {
        continue
      }
      res.set(parts[i], true)
    }
    return res
  }

  // encode only keys with true value
  encodeExpand(mute: Map<string, boolean>): string {
    let res: string[] = []
    for (let k of mute.keys()) {
      if (mute.get(k)) {
        res.push(k)
      }
    }
    return res.join("!")
  }
}

let instance: SettingsStore

export function setSettingsStore(store: SettingsStore) {
  instance = store
}

export function getSettingsStore(): SettingsStore {
  return instance
}
