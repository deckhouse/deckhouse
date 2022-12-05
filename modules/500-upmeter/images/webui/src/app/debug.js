import * as d3 from "d3"

export function setupDebug() {
  window.enableDebug = function () {
    window.d3 = d3
  }
}
