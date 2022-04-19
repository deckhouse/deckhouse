import React from "react"
import ReactDOM from "react-dom"
import { getTheme, ThemeContext } from "@grafana/ui"

// CSS
import "../css/main.css"
import "bootstrap/dist/css/bootstrap.min.css"
import "@grafana/ui/index.scss"
import "./index.scss"

// 3rd party libs
import "./libs/fontawesome"

// Components
import { TopNavBar } from "./components/TopNavBar"

// Services
import { EventsSrv, getEventsSrv, setEventsSrv } from "./services/EventsSrv"
import { setSettingsStore, SettingsStore } from "./services/SettingsStore"
import { getTimeRangeSrv, setTimeRangeSrv, TimeRangeSrv } from "./services/TimeRangeSrv"
import { DatasetSrv, getDatasetSrv, setDatasetSrv } from "./services/DatasetSrv"

import { setupDebug } from "./debug"

function initApp() {
  // Instantiate Services
  setSettingsStore(new SettingsStore())
  setEventsSrv(new EventsSrv())
  setTimeRangeSrv(new TimeRangeSrv())
  setDatasetSrv(new DatasetSrv())

  // Setup Listeners for graph render.
  getEventsSrv().listenEvent("UpdateGraph", "main", (data: any) => {
    getDatasetSrv().requestGroups()
  })
  getEventsSrv().listenEvent("UpdateGroupProbes", "main", ({ group, settings }) => {
    getDatasetSrv().requestGroupProbesData(group)
  })
}

export function startApp() {
  let light = getTheme("light")

  // Load settings from address bar.
  getTimeRangeSrv().init()

  let muteSelection = getTimeRangeSrv().getMuteSelection()
  ReactDOM.render(
    <ThemeContext.Provider value={light}>
      <TopNavBar muteSelection={muteSelection} />
    </ThemeContext.Provider>,
    document.getElementById("topNavBar"),
  )

  getEventsSrv().fireEvent("UpdateGraph")
}

document.addEventListener("DOMContentLoaded", function () {
  initApp()
  startApp()
  setupDebug()
})

// 'Back' button
window.onpopstate = function (event: any) {
  // Load time range and mute selection from address bar,
  // refresh TopNavBar and re-render graph.
  getTimeRangeSrv().init()
  getEventsSrv().fireEvent("Refresh")
  getEventsSrv().fireEvent("UpdateGraph")
}
