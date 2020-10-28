import * as dom from "./dom";
import {langPack} from "../i18n/en";

export const updatePage = function(type, obj) {
  if (type === "fetch start") {
    loadingIndicator(true);
    return
  } else {
    loadingIndicator(false);
  }
  if (type === "fetch error") {
    alertError(true, obj.message);
    return
  }
  if (checkStatusesJson(obj)) {
    alertError(false);
    updateStatuses(obj);
  } else {
    alertError(true, "Bad json fetched");
  }
}

const loadingIndicator = function(show) {
  let el = dom.getFirstByClassName(document, "summary-related load-indicator")
  dom.classed(el, "hidden", !show)
}

const alertError = function(show, error) {
  // Show/hide alert
  let el = dom.getFirstByClassName(document, "load-message")
  if (show) {
    dom.html(el, error)
  } else {
    dom.html(el, '')
  }
  dom.classed(el, "alert alert-danger", show);
  dom.classed(el, "hidden", !show);
  // Show/hide fade panel and big indicator for error
  el = dom.getFirstByClassName(document, "error-related load-indicator")
  dom.classed(el, "hidden", !show)
  el = dom.getFirstByClassName(document, "error-related fade-panel")
  dom.classed(el, "hidden", !show)
}

const fadeStatuses = function(fade) {
  let summary = dom.getFirstByClassName(document, "top-summary")
  if (!dom.hasClass(summary, "alert-secondary")) {
    dom.classed(summary, "s-fade", fade)
  }

  let statuses = dom.getFirstByClassName(document, "statuses")
  dom.classed(statuses, "s-fade", fade)

}

const checkStatusesJson = function(obj) {
  if (!obj.hasOwnProperty("status")) {
    return false;
  }
  if (!obj.hasOwnProperty("rows")) {
    return false
  }
  if (!Array.isArray(obj["rows"])) {
    return false
  }
  return true;
}

const updateStatuses = function(obj) {
  setSummaryStatus(obj["status"])

  let tableEl = dom.getFirstByClassName(document, "statuses")
  if (typeof tableEl === "undefined") {
    return undefined
  }
  obj["rows"].forEach(function(row) {
    updateOrCreateRow(tableEl, row)
  })
}

const setSummaryStatus = function(status) {
  let summaryEl = dom.getFirstByClassName(document, "top-summary")
  let labelEl = dom.getFirstByClassName(summaryEl, "summary-label")
  if (typeof labelEl === "undefined") {
    return
  }
  dom.offClass(labelEl, textClasses)
  dom.onClass(labelEl, textClass(status))
  labelEl.innerHTML = `Cluster ${status}`
}

const updateOrCreateRow = function(tableEl, row) {
  let groupClass = "group-" + row.group
  let groupLabel = langPack.groups[row.group].label;
  let groupDesc = langPack.groups[row.group].description;
  let statusClass = textClass(row.status)
  let statusLabel = row.status;

  let rowEl = dom.getFirstByClassName(tableEl, groupClass)
  if (typeof rowEl === "undefined") {
    let html = `
   <div class="alert d-flex group-${row.group}"><!-- class align-items-center  -->
      <div>
        <div class="group-label">${groupLabel}</div>
        <div class="group-description">${groupDesc}</div>
      </div>
      <div class="status-label ${statusClass}">${statusLabel}</div>
    </div>
  `;
    tableEl.insertAdjacentHTML("beforeend", html)
    return
  }

  let groupEl = dom.getFirstByClassName(rowEl, "group-label")
  groupEl.innerHTML = groupLabel;
  let statusEl = dom.getFirstByClassName(rowEl, "status-label")
  statusEl.innerHTML = statusLabel;
  dom.offClass(statusEl, textClasses)
  dom.onClass(statusEl, statusClass)
}

const alertClasses = "alert-secondary alert-success alert-warning alert-danger"
const alertClass = function(status) {
  if (status === "Operational") {
    return "alert-success"
  }
  if (status === "Degraded") {
    return "alert-warning"
  }
  if (status === "Outage") {
    return "alert-danger"
  }
}
const textClasses = "text-secondary text-success text-warning text-danger"
const textClass = function(status) {
  if (status === "Operational") {
    return "text-success"
  }
  if (status === "Degraded") {
    return "text-warning"
  }
  if (status === "Outage") {
    return "text-danger"
  }
  return ""
}
