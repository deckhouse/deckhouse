function scrollFunc(params) {
  // WARNING: DOES NOT SCROLL LEFT
  params.container.scrollTo(params.element, params.offset, params.duration);
  return;
}

function isDefined(arg) {
  return typeof arg !== 'undefined';
}

export default function nxnScrollContainerToElement(params) {
  if (!isDefined(params.container) || !isDefined(params.element)) {
    throw 'Error: container and element are mandatory';
  }
  params.duration = isDefined(params.duration) ? params.duration : 0;
  params.offset = isDefined(params.offset) ? params.offset : 0;
  params.overflow_mode = isDefined(params.overflow_mode) ? params.overflow_mode : false;
  if (params.element.length && params.container.length) {
    if (params.overflow_mode) {
      var c_oh = params.container.prop('offsetHeight');
      var c_st = params.container.prop('scrollTop');
      var e_ot = params.element.prop('offsetTop');
      var e_oh = params.element.prop('offsetHeight');
      if (c_st < (e_ot + e_oh) && (e_ot + e_oh) < c_st + c_oh) {
        // already there
      } else {
        scrollFunc(params);
      }
    } else {
      scrollFunc(params);
    }
  }
}
