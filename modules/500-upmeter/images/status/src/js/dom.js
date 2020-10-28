export const hasClass = function(el, cl) {
  if (notDefined(el)) { return }
  if (notDefined(cl)) { return }
  if (el.classList) {
    return el.classList.contains(cl);
  }
  else {
    return new RegExp('\\b' + cl + '\\b').test(el.className)
  }
}

export const addClass = function(el, cl) {
  if (notDefined(el)) { return }
  if (notDefined(cl)) { return }
  let a = cl.split(' ');
  for (let i = 0; i < a.length; i++) {
    if (el.classList) {
      el.classList.add(a[i]);
    }
    else {
      el.className += ' ' + a[i];
    }
  }
}

export const removeClass = function(el, cl) {
  if (notDefined(el)) { return }
  if (notDefined(cl)) { return }
  let a = cl.split(' ');
  for (let i = 0; i < a.length; i++) {
    if (el.classList) {
      el.classList.remove(a[i]);
    }
    else {
      el.className = el.className.replace(new RegExp('\\b'+ a[i] +'\\b', 'g'), '');
    }
  }
}

export const toggleClass = function(el, cl) {
  if (notDefined(el)) { return }
  if (notDefined(cl)) { return }
  let a = cl.split(' ');

  for (let i = 0; i < a.length; i++) {
    if (el.classList) {
      el.classList.toggle(a[i]);
    }
    else {
      if (hasClass(el, a[i])) {
        el.className = el.className.replace(new RegExp('\\b'+ a[i] +'\\b', 'g'), '');
      } else {
        el.className += ' ' + a[i];
      }
    }
  }
}

export const classed = function(el, cl, on) {
  if (notDefined(el)) { return }
  if (notDefined(cl)) { return }
  let a = cl.split(' ');
  for (let i = 0; i < a.length; i++) {
    if (on && !hasClass(el, a[i])) {
      addClass(el, a[i]);
    }
    if (!on && hasClass(el, a[i])) {
      removeClass(el, a[i])
    }
  }
}

export const onClass = function(el, cl) {
  classed(el, cl, true);
}

export const offClass = function(el, cl) {
  classed(el, cl, false)
}

export const getFirstByClassName = function(el, name) {
  if (notDefined(el)) { return }
  let els = el.getElementsByClassName(name)
  if (els.length > 0) {
    return els[0]
  }
}

export const html = function(el, html) {
  if (notDefined(el)) { return }
  el.innerHTML = html
}

export const text = function(el, text) {
  if (notDefined(el)) { return }
  el.innerText = text
}

const notDefined = function(obj) {
  return typeof obj === "undefined"
}
