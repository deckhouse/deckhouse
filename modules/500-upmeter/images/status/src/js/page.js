import * as dom from "./dom"
import { group } from "../i18n/en"

export const updatePage = function (type, obj) {
  if (type === "fetch start") {
    loadingIndicator(true)
    return
  }

  loadingIndicator(false)

  if (type === "fetch error") {
    alertError(true, obj.message)
    return
  }

  if (checkStatusesJson(obj)) {
    alertError(false)
    updateStatuses(obj)
    return
  }

  alertError(true, "Bad json fetched")
}

const loadingIndicator = function (show) {
  let el = dom.getFirstByClassName(document, "summary-related load-indicator")
  dom.classed(el, "hidden", !show)
}

const alertError = function (show, error) {
  // Show/hide alert
  let el = dom.getFirstByClassName(document, "load-message")
  if (show) {
    dom.html(el, error)
  } else {
    dom.html(el, "")
  }
  dom.classed(el, "alert alert-danger", show)
  dom.classed(el, "hidden", !show)
  // Show/hide fade panel and big indicator for error
  el = dom.getFirstByClassName(document, "error-related load-indicator")
  dom.classed(el, "hidden", !show)
  el = dom.getFirstByClassName(document, "error-related fade-panel")
  dom.classed(el, "hidden", !show)
}

const checkStatusesJson = function (obj) {
  return obj.status && Array.isArray(obj.rows)
}

const updateStatuses = function (obj) {
  setSummaryStatus(obj.status)

  const tableEl = dom.getFirstByClassName(document, "statuses")
  if (!tableEl) {
    return
  }
  obj.rows.forEach((row) => {
    updateOrCreateRow(tableEl, row)
  })
}

const setSummaryStatus = function (status) {
  const summaryEl = dom.getFirstByClassName(document, "top-summary")
  const labelEl = dom.getFirstByClassName(summaryEl, "summary-label")
  if (!labelEl) {
    return
  }
  dom.offClass(labelEl, textClasses)
  dom.onClass(labelEl, textClass(status))
  labelEl.innerHTML = `Cluster ${status}`
}

const updateOrCreateRow = function (tableEl, row) {
  let groupClass = `group-${row.group}`
  let statusClass = textClass(row.status)
  let statusLabel = row.status
  let { label, description } = group(row.group)

  const rowEl = dom.getFirstByClassName(tableEl, groupClass)
  if (typeof rowEl === "undefined") {
    const html = `
      <div class="alert d-flex group-${row.group}"><!-- class align-items-center  -->
        <div>
          <div class="group-label">${label}</div>
          <div class="group-description">${description}</div>
        </div>
        <div class="status-label ${statusClass}">${statusLabel}</div>
      </div> `

    tableEl.insertAdjacentHTML("beforeend", html)
    return
  }

  const groupEl = dom.getFirstByClassName(rowEl, "group-label")
  groupEl.innerHTML = label
  const statusEl = dom.getFirstByClassName(rowEl, "status-label")
  statusEl.innerHTML = statusLabel
  dom.offClass(statusEl, textClasses)
  dom.onClass(statusEl, statusClass)
}

const textClasses = "text-secondary text-success text-warning text-danger"
const textClass = function (status) {
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
