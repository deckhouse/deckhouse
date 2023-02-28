export class EventsSrv {
  callbacks: Map<string, Map<string, (data?: any) => void>>

  constructor() {
    this.callbacks = new Map<string, Map<string, (data?: any) => void>>()
  }

  fireEvent(eventName: string, data?: any) {
    if (!this.callbacks.has(eventName)) {
      return
    }
    let evCallbacks = this.callbacks.get(eventName)
    for (let id of evCallbacks.keys()) {
      let cb = evCallbacks.get(id)
      cb(data)
    }
  }

  listenEvent(eventName: string, id: string, callback: (data?: any) => void) {
    if (!this.callbacks.has(eventName)) {
      this.callbacks.set(eventName, new Map<string, (data?: any) => void>())
    }

    this.callbacks.get(eventName).set(id, callback)
  }

  /**
   * @param {string} eventName name of event
   * @param {string} id callback identifier
   */
  unlistenEvent(eventName: string, id: string) {
    if (!this.callbacks.has(eventName)) {
      return
    }
    if (this.callbacks.get(eventName).has(id)) {
      this.callbacks.get(eventName).delete(id)
    }
  }
}

let instance: EventsSrv

export function setEventsSrv(srv: EventsSrv) {
  instance = srv
}

export function getEventsSrv(): EventsSrv {
  return instance
}
