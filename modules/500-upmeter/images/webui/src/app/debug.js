import * as d3 from './libs/d3';

export function setupDebug() {
  window.enableDebug = function() {
    window.d3 = d3;
  }
}
