export function hasClass(el, cl) {
  if (notDefined(el)) {
    return
  }
  if (notDefined(cl)) {
    return
  }
  if (el.classList) {
    return el.classList.contains(cl)
  }
  return new RegExp(`\\b${cl}\\b`).test(el.className)
}

export function addClass(el, cl) {
  if (notDefined(el)) {
    return
  }
  if (notDefined(cl)) {
    return
  }
  const a = cl.split(" ")
  for (let i = 0; i < a.length; i++) {
    if (el.classList) {
      el.classList.add(a[i])
    } else {
      el.className += ` ${a[i]}`
    }
  }
}

export function removeClass(el, cl) {
  if (notDefined(el)) {
    return
  }
  if (notDefined(cl)) {
    return
  }
  const a = cl.split(" ")
  for (let i = 0; i < a.length; i++) {
    if (el.classList) {
      el.classList.remove(a[i])
    } else {
      el.className = el.className.replace(new RegExp(`\\b${a[i]}\\b`, "g"), "")
    }
  }
}

export function toggleClass(el, cl) {
  if (notDefined(el)) {
    return
  }
  if (notDefined(cl)) {
    return
  }
  const a = cl.split(" ")

  for (let i = 0; i < a.length; i++) {
    if (el.classList) {
      el.classList.toggle(a[i])
    } else if (hasClass(el, a[i])) {
      el.className = el.className.replace(new RegExp(`\\b${a[i]}\\b`, "g"), "")
    } else {
      el.className += ` ${a[i]}`
    }
  }
}

export function classed(el, cl, on) {
  if (notDefined(el)) {
    return
  }
  if (notDefined(cl)) {
    return
  }
  const a = cl.split(" ")
  for (let i = 0; i < a.length; i++) {
    if (on && !hasClass(el, a[i])) {
      addClass(el, a[i])
    }
    if (!on && hasClass(el, a[i])) {
      removeClass(el, a[i])
    }
  }
}

export function onClass(el, cl) {
  classed(el, cl, true)
}

export function offClass(el, cl) {
  classed(el, cl, false)
}

export const getFirstByClassName = function (el, name) {
  if (notDefined(el)) {
    return
  }
  const els = el.getElementsByClassName(name)
  if (els.length > 0) {
    return els[0]
  }
}

export const html = function (el, html) {
  if (notDefined(el)) {
    return
  }
  el.innerHTML = html
}

export const text = function (el, text) {
  if (notDefined(el)) {
    return
  }
  el.innerText = text
}

const notDefined = function (obj) {
  return typeof obj === "undefined"
}
