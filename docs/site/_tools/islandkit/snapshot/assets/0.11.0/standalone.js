//#region ../../node_modules/@vue/shared/dist/shared.esm-bundler.js
// @__NO_SIDE_EFFECTS__
function e(e) {
	let t = /* @__PURE__ */ Object.create(null);
	for (let n of e.split(",")) t[n] = 1;
	return (e) => e in t;
}
var t = {}, n = [], r = () => {}, i = () => !1, a = (e) => e.charCodeAt(0) === 111 && e.charCodeAt(1) === 110 && (e.charCodeAt(2) > 122 || e.charCodeAt(2) < 97), o = (e) => e.startsWith("onUpdate:"), s = Object.assign, c = (e, t) => {
	let n = e.indexOf(t);
	n > -1 && e.splice(n, 1);
}, l = Object.prototype.hasOwnProperty, u = (e, t) => l.call(e, t), d = Array.isArray, f = (e) => x(e) === "[object Map]", p = (e) => x(e) === "[object Set]", m = (e) => x(e) === "[object Date]", h = (e) => typeof e == "function", g = (e) => typeof e == "string", _ = (e) => typeof e == "symbol", v = (e) => typeof e == "object" && !!e, y = (e) => (v(e) || h(e)) && h(e.then) && h(e.catch), b = Object.prototype.toString, x = (e) => b.call(e), ee = (e) => x(e).slice(8, -1), S = (e) => x(e) === "[object Object]", te = (e) => g(e) && e !== "NaN" && e[0] !== "-" && "" + parseInt(e, 10) === e, ne = /* @__PURE__ */ e(",key,ref,ref_for,ref_key,onVnodeBeforeMount,onVnodeMounted,onVnodeBeforeUpdate,onVnodeUpdated,onVnodeBeforeUnmount,onVnodeUnmounted"), re = (e) => {
	let t = /* @__PURE__ */ Object.create(null);
	return ((n) => t[n] || (t[n] = e(n)));
}, ie = /-\w/g, C = re((e) => e.replace(ie, (e) => e.slice(1).toUpperCase())), ae = /\B([A-Z])/g, oe = re((e) => e.replace(ae, "-$1").toLowerCase()), se = re((e) => e.charAt(0).toUpperCase() + e.slice(1)), ce = re((e) => e ? `on${se(e)}` : ""), le = (e, t) => !Object.is(e, t), ue = (e, ...t) => {
	for (let n = 0; n < e.length; n++) e[n](...t);
}, de = (e, t, n, r = !1) => {
	Object.defineProperty(e, t, {
		configurable: !0,
		enumerable: !1,
		writable: r,
		value: n
	});
}, fe = (e) => {
	let t = parseFloat(e);
	return isNaN(t) ? e : t;
}, pe, me = () => pe || (pe = typeof globalThis < "u" ? globalThis : typeof self < "u" ? self : typeof window < "u" ? window : typeof global < "u" ? global : {});
function he(e) {
	if (d(e)) {
		let t = {};
		for (let n = 0; n < e.length; n++) {
			let r = e[n], i = g(r) ? ye(r) : he(r);
			if (i) for (let e in i) t[e] = i[e];
		}
		return t;
	}
	if (g(e) || v(e)) return e;
}
var ge = /;(?![^(]*\))/g, _e = /:([^]+)/, ve = /\/\*[^]*?\*\//g;
function ye(e) {
	let t = {};
	return e.replace(ve, "").split(ge).forEach((e) => {
		if (e) {
			let n = e.split(_e);
			n.length > 1 && (t[n[0].trim()] = n[1].trim());
		}
	}), t;
}
function w(e) {
	let t = "";
	if (g(e)) t = e;
	else if (d(e)) for (let n = 0; n < e.length; n++) {
		let r = w(e[n]);
		r && (t += r + " ");
	}
	else if (v(e)) for (let n in e) e[n] && (t += n + " ");
	return t.trim();
}
var be = "itemscope,allowfullscreen,formnovalidate,ismap,nomodule,novalidate,readonly", xe = /* @__PURE__ */ e(be);
be + "";
function Se(e) {
	return !!e || e === "";
}
function Ce(e, t) {
	if (e.length !== t.length) return !1;
	let n = !0;
	for (let r = 0; n && r < e.length; r++) n = we(e[r], t[r]);
	return n;
}
function we(e, t) {
	if (e === t) return !0;
	let n = m(e), r = m(t);
	if (n || r) return n && r ? e.getTime() === t.getTime() : !1;
	if (n = _(e), r = _(t), n || r) return e === t;
	if (n = d(e), r = d(t), n || r) return n && r ? Ce(e, t) : !1;
	if (n = v(e), r = v(t), n || r) {
		if (!n || !r || Object.keys(e).length !== Object.keys(t).length) return !1;
		for (let n in e) {
			let r = e.hasOwnProperty(n), i = t.hasOwnProperty(n);
			if (r && !i || !r && i || !we(e[n], t[n])) return !1;
		}
	}
	return String(e) === String(t);
}
var Te = (e) => !!(e && e.__v_isRef === !0), T = (e) => g(e) ? e : e == null ? "" : d(e) || v(e) && (e.toString === b || !h(e.toString)) ? Te(e) ? T(e.value) : JSON.stringify(e, Ee, 2) : String(e), Ee = (e, t) => Te(t) ? Ee(e, t.value) : f(t) ? { [`Map(${t.size})`]: [...t.entries()].reduce((e, [t, n], r) => (e[De(t, r) + " =>"] = n, e), {}) } : p(t) ? { [`Set(${t.size})`]: [...t.values()].map((e) => De(e)) } : _(t) ? De(t) : v(t) && !d(t) && !S(t) ? String(t) : t, De = (e, t = "") => {
	var n;
	return _(e) ? `Symbol(${(n = e.description) == null ? t : n})` : e;
}, E, Oe = class {
	constructor(e = !1) {
		this.detached = e, this._active = !0, this._on = 0, this.effects = [], this.cleanups = [], this._isPaused = !1, this._warnOnRun = !0, this.__v_skip = !0, !e && E && (E.active ? (this.parent = E, this.index = (E.scopes || (E.scopes = [])).push(this) - 1) : (this._active = !1, this._warnOnRun = !1));
	}
	get active() {
		return this._active;
	}
	pause() {
		if (this._active) {
			this._isPaused = !0;
			let e, t;
			if (this.scopes) {
				let n = this.scopes.slice();
				for (e = 0, t = n.length; e < t; e++) n[e].pause();
			}
			for (e = 0, t = this.effects.length; e < t; e++) this.effects[e].pause();
		}
	}
	resume() {
		if (this._active && this._isPaused) {
			this._isPaused = !1;
			let e, t;
			if (this.scopes) {
				let n = this.scopes.slice();
				for (e = 0, t = n.length; e < t; e++) n[e].resume();
			}
			let n = this.effects.slice();
			for (e = 0, t = n.length; e < t; e++) n[e].resume();
		}
	}
	run(e) {
		if (this._active) {
			let t = E;
			try {
				return E = this, e();
			} finally {
				E = t;
			}
		}
	}
	on() {
		++this._on === 1 && (this.prevScope = E, E = this);
	}
	off() {
		if (this._on > 0 && --this._on === 0) {
			if (E === this) E = this.prevScope;
			else {
				let e = E;
				for (; e;) {
					if (e.prevScope === this) {
						e.prevScope = this.prevScope;
						break;
					}
					e = e.prevScope;
				}
			}
			this.prevScope = void 0;
		}
	}
	stop(e) {
		if (this._active) {
			this._active = !1;
			let t, n;
			for (t = 0, n = this.effects.length; t < n; t++) this.effects[t].stop();
			for (this.effects.length = 0, t = 0, n = this.cleanups.length; t < n; t++) this.cleanups[t]();
			if (this.cleanups.length = 0, this.scopes) {
				let e = this.scopes.slice();
				for (t = 0, n = e.length; t < n; t++) e[t].stop(!0);
				this.scopes.length = 0;
			}
			if (!this.detached && this.parent && !e) {
				let e = this.parent.scopes.pop();
				e && e !== this && (this.parent.scopes[this.index] = e, e.index = this.index);
			}
			this.parent = void 0;
		}
	}
};
function ke() {
	return E;
}
function Ae(e, t = !1) {
	E && E.cleanups.push(e);
}
var D, je = /* @__PURE__ */ new WeakSet(), Me = class {
	constructor(e) {
		this.fn = e, this.deps = void 0, this.depsTail = void 0, this.flags = 5, this.next = void 0, this.cleanup = void 0, this.scheduler = void 0, E && (E.active ? E.effects.push(this) : this.flags &= -2);
	}
	pause() {
		this.flags |= 64;
	}
	resume() {
		this.flags & 64 && (this.flags &= -65, je.has(this) && (je.delete(this), this.trigger()));
	}
	notify() {
		this.flags & 2 && !(this.flags & 32) || this.flags & 8 || Ie(this);
	}
	run() {
		if (!(this.flags & 1)) return this.fn();
		this.flags |= 2, Ye(this), ze(this);
		let e = D, t = Ge;
		D = this, Ge = !0;
		try {
			return this.fn();
		} finally {
			Be(this), D = e, Ge = t, this.flags &= -3;
		}
	}
	stop() {
		if (this.flags & 1) {
			for (let e = this.deps; e; e = e.nextDep) Ue(e);
			this.deps = this.depsTail = void 0, Ye(this), this.onStop && this.onStop(), this.flags &= -2;
		}
	}
	trigger() {
		this.flags & 64 ? je.add(this) : this.scheduler ? this.scheduler() : this.runIfDirty();
	}
	runIfDirty() {
		Ve(this) && this.run();
	}
	get dirty() {
		return Ve(this);
	}
}, Ne = 0, Pe, Fe;
function Ie(e, t = !1) {
	if (e.flags |= 8, t) {
		e.next = Fe, Fe = e;
		return;
	}
	e.next = Pe, Pe = e;
}
function Le() {
	Ne++;
}
function Re() {
	if (--Ne > 0) return;
	if (Fe) {
		let e = Fe;
		for (Fe = void 0; e;) {
			let t = e.next;
			e.next = void 0, e.flags &= -9, e = t;
		}
	}
	let e;
	for (; Pe;) {
		let t = Pe;
		for (Pe = void 0; t;) {
			let n = t.next;
			if (t.next = void 0, t.flags &= -9, t.flags & 1) try {
				t.trigger();
			} catch (t) {
				e || (e = t);
			}
			t = n;
		}
	}
	if (e) throw e;
}
function ze(e) {
	for (let t = e.deps; t; t = t.nextDep) t.version = -1, t.prevActiveLink = t.dep.activeLink, t.dep.activeLink = t;
}
function Be(e) {
	let t, n = e.depsTail, r = n;
	for (; r;) {
		let e = r.prevDep;
		r.version === -1 ? (r === n && (n = e), Ue(r), We(r)) : t = r, r.dep.activeLink = r.prevActiveLink, r.prevActiveLink = void 0, r = e;
	}
	e.deps = t, e.depsTail = n;
}
function Ve(e) {
	for (let t = e.deps; t; t = t.nextDep) if (t.dep.version !== t.version || t.dep.computed && (He(t.dep.computed) || t.dep.version !== t.version)) return !0;
	return !!e._dirty;
}
function He(e) {
	if (e.flags & 4 && !(e.flags & 16) || (e.flags &= -17, e.globalVersion === Xe) || (e.globalVersion = Xe, !e.isSSR && e.flags & 128 && (!e.deps && !e._dirty || !Ve(e)))) return;
	e.flags |= 2;
	let t = e.dep, n = D, r = Ge;
	D = e, Ge = !0;
	try {
		ze(e);
		let n = e.fn(e._value);
		(t.version === 0 || le(n, e._value)) && (e.flags |= 128, e._value = n, t.version++);
	} catch (e) {
		throw t.version++, e;
	} finally {
		D = n, Ge = r, Be(e), e.flags &= -3;
	}
}
function Ue(e, t = !1) {
	let { dep: n, prevSub: r, nextSub: i } = e;
	if (r && (r.nextSub = i, e.prevSub = void 0), i && (i.prevSub = r, e.nextSub = void 0), n.subs === e && (n.subs = r, !r && n.computed)) {
		n.computed.flags &= -5;
		for (let e = n.computed.deps; e; e = e.nextDep) Ue(e, !0);
	}
	!t && !--n.sc && n.map && n.map.delete(n.key);
}
function We(e) {
	let { prevDep: t, nextDep: n } = e;
	t && (t.nextDep = n, e.prevDep = void 0), n && (n.prevDep = t, e.nextDep = void 0);
}
var Ge = !0, Ke = [];
function qe() {
	Ke.push(Ge), Ge = !1;
}
function Je() {
	let e = Ke.pop();
	Ge = e === void 0 || e;
}
function Ye(e) {
	let { cleanup: t } = e;
	if (e.cleanup = void 0, t) {
		let e = D;
		D = void 0;
		try {
			t();
		} finally {
			D = e;
		}
	}
}
var Xe = 0, Ze = class {
	constructor(e, t) {
		this.sub = e, this.dep = t, this.version = t.version, this.nextDep = this.prevDep = this.nextSub = this.prevSub = this.prevActiveLink = void 0;
	}
}, Qe = class {
	constructor(e) {
		this.computed = e, this.version = 0, this.activeLink = void 0, this.subs = void 0, this.map = void 0, this.key = void 0, this.sc = 0, this.__v_skip = !0;
	}
	track(e) {
		if (!D || !Ge || D === this.computed) return;
		let t = this.activeLink;
		if (t === void 0 || t.sub !== D) t = this.activeLink = new Ze(D, this), D.deps ? (t.prevDep = D.depsTail, D.depsTail.nextDep = t, D.depsTail = t) : D.deps = D.depsTail = t, $e(t);
		else if (t.version === -1 && (t.version = this.version, t.nextDep)) {
			let e = t.nextDep;
			e.prevDep = t.prevDep, t.prevDep && (t.prevDep.nextDep = e), t.prevDep = D.depsTail, t.nextDep = void 0, D.depsTail.nextDep = t, D.depsTail = t, D.deps === t && (D.deps = e);
		}
		return t;
	}
	trigger(e) {
		this.version++, Xe++, this.notify(e);
	}
	notify(e) {
		Le();
		try {
			for (let e = this.subs; e; e = e.prevSub) e.sub.notify() && e.sub.dep.notify();
		} finally {
			Re();
		}
	}
};
function $e(e) {
	if (e.dep.sc++, e.sub.flags & 4) {
		let t = e.dep.computed;
		if (t && !e.dep.subs) {
			t.flags |= 20;
			for (let e = t.deps; e; e = e.nextDep) $e(e);
		}
		let n = e.dep.subs;
		n !== e && (e.prevSub = n, n && (n.nextSub = e)), e.dep.subs = e;
	}
}
var et = /* @__PURE__ */ new WeakMap(), tt = /* @__PURE__ */ Symbol(""), nt = /* @__PURE__ */ Symbol(""), rt = /* @__PURE__ */ Symbol("");
function O(e, t, n) {
	if (Ge && D) {
		let t = et.get(e);
		t || et.set(e, t = /* @__PURE__ */ new Map());
		let r = t.get(n);
		r || (t.set(n, r = new Qe()), r.map = t, r.key = n), r.track();
	}
}
function it(e, t, n, r, i, a) {
	let o = et.get(e);
	if (!o) {
		Xe++;
		return;
	}
	let s = (e) => {
		e && e.trigger();
	};
	if (Le(), t === "clear") o.forEach(s);
	else {
		let i = d(e), a = i && te(n);
		if (i && n === "length") {
			let e = Number(r);
			o.forEach((t, n) => {
				(n === "length" || n === rt || !_(n) && n >= e) && s(t);
			});
		} else switch ((n !== void 0 || o.has(void 0)) && s(o.get(n)), a && s(o.get(rt)), t) {
			case "add":
				i ? a && s(o.get("length")) : (s(o.get(tt)), f(e) && s(o.get(nt)));
				break;
			case "delete":
				i || (s(o.get(tt)), f(e) && s(o.get(nt)));
				break;
			case "set": f(e) && s(o.get(tt));
		}
	}
	Re();
}
function at(e) {
	let t = /* @__PURE__ */ A(e);
	return t === e ? t : (O(t, "iterate", rt), /* @__PURE__ */ k(e) ? t : t.map(j));
}
function ot(e) {
	return O(e = /* @__PURE__ */ A(e), "iterate", rt), e;
}
function st(e, t) {
	return /* @__PURE__ */ Ut(e) ? Kt(/* @__PURE__ */ Ht(e) ? j(t) : t) : j(t);
}
var ct = {
	__proto__: null,
	[Symbol.iterator]() {
		return lt(this, Symbol.iterator, (e) => st(this, e));
	},
	concat(...e) {
		return at(this).concat(...e.map((e) => d(e) ? at(e) : e));
	},
	entries() {
		return lt(this, "entries", (e) => (e[1] = st(this, e[1]), e));
	},
	every(e, t) {
		return dt(this, "every", e, t, void 0, arguments);
	},
	filter(e, t) {
		return dt(this, "filter", e, t, (e) => e.map((e) => st(this, e)), arguments);
	},
	find(e, t) {
		return dt(this, "find", e, t, (e) => st(this, e), arguments);
	},
	findIndex(e, t) {
		return dt(this, "findIndex", e, t, void 0, arguments);
	},
	findLast(e, t) {
		return dt(this, "findLast", e, t, (e) => st(this, e), arguments);
	},
	findLastIndex(e, t) {
		return dt(this, "findLastIndex", e, t, void 0, arguments);
	},
	forEach(e, t) {
		return dt(this, "forEach", e, t, void 0, arguments);
	},
	includes(...e) {
		return pt(this, "includes", e);
	},
	indexOf(...e) {
		return pt(this, "indexOf", e);
	},
	join(e) {
		return at(this).join(e);
	},
	lastIndexOf(...e) {
		return pt(this, "lastIndexOf", e);
	},
	map(e, t) {
		return dt(this, "map", e, t, void 0, arguments);
	},
	pop() {
		return mt(this, "pop");
	},
	push(...e) {
		return mt(this, "push", e);
	},
	reduce(e, ...t) {
		return ft(this, "reduce", e, t);
	},
	reduceRight(e, ...t) {
		return ft(this, "reduceRight", e, t);
	},
	shift() {
		return mt(this, "shift");
	},
	some(e, t) {
		return dt(this, "some", e, t, void 0, arguments);
	},
	splice(...e) {
		return mt(this, "splice", e);
	},
	toReversed() {
		return at(this).toReversed();
	},
	toSorted(e) {
		return at(this).toSorted(e);
	},
	toSpliced(...e) {
		return at(this).toSpliced(...e);
	},
	unshift(...e) {
		return mt(this, "unshift", e);
	},
	values() {
		return lt(this, "values", (e) => st(this, e));
	}
};
function lt(e, t, n) {
	let r = ot(e), i = r[t]();
	return r !== e && !/* @__PURE__ */ k(e) && (i._next = i.next, i.next = () => {
		let e = i._next();
		return e.done || (e.value = n(e.value)), e;
	}), i;
}
var ut = Array.prototype;
function dt(e, t, n, r, i, a) {
	let o = ot(e), s = o !== e && !/* @__PURE__ */ k(e), c = o[t];
	if (c !== ut[t]) {
		let t = c.apply(e, a);
		return s ? j(t) : t;
	}
	let l = n;
	o !== e && (s ? l = function(t, r) {
		return n.call(this, st(e, t), r, e);
	} : n.length > 2 && (l = function(t, r) {
		return n.call(this, t, r, e);
	}));
	let u = c.call(o, l, r);
	return s && i ? i(u) : u;
}
function ft(e, t, n, r) {
	let i = ot(e), a = i !== e && !/* @__PURE__ */ k(e), o = n, s = !1;
	i !== e && (a ? (s = r.length === 0, o = function(t, r, i) {
		return s && (s = !1, t = st(e, t)), n.call(this, t, st(e, r), i, e);
	}) : n.length > 3 && (o = function(t, r, i) {
		return n.call(this, t, r, i, e);
	}));
	let c = i[t](o, ...r);
	return s ? st(e, c) : c;
}
function pt(e, t, n) {
	let r = /* @__PURE__ */ A(e);
	O(r, "iterate", rt);
	let i = r[t](...n);
	return (i === -1 || i === !1) && /* @__PURE__ */ Wt(n[0]) ? (n[0] = /* @__PURE__ */ A(n[0]), r[t](...n)) : i;
}
function mt(e, t, n = []) {
	qe(), Le();
	let r = (/* @__PURE__ */ A(e))[t].apply(e, n);
	return Re(), Je(), r;
}
var ht = /* @__PURE__ */ e("__proto__,__v_isRef,__isVue"), gt = new Set(/* @__PURE__ */ Object.getOwnPropertyNames(Symbol).filter((e) => e !== "arguments" && e !== "caller").map((e) => Symbol[e]).filter(_));
function _t(e) {
	_(e) || (e = String(e));
	let t = /* @__PURE__ */ A(this);
	return O(t, "has", e), t.hasOwnProperty(e);
}
var vt = class {
	constructor(e = !1, t = !1) {
		this._isReadonly = e, this._isShallow = t;
	}
	get(e, t, n) {
		if (t === "__v_skip") return e.__v_skip;
		let r = this._isReadonly, i = this._isShallow;
		if (t === "__v_isReactive") return !r;
		if (t === "__v_isReadonly") return r;
		if (t === "__v_isShallow") return i;
		if (t === "__v_raw") return n === (r ? i ? It : Ft : i ? Pt : Nt).get(e) || Object.getPrototypeOf(e) === Object.getPrototypeOf(n) ? e : void 0;
		let a = d(e);
		if (!r) {
			let e;
			if (a && (e = ct[t])) return e;
			if (t === "hasOwnProperty") return _t;
		}
		let o = Reflect.get(e, t, /* @__PURE__ */ M(e) ? e : n);
		if ((_(t) ? gt.has(t) : ht(t)) || (r || O(e, "get", t), i)) return o;
		if (/* @__PURE__ */ M(o)) {
			let e = a && te(t) ? o : o.value;
			return r && v(e) ? /* @__PURE__ */ Bt(e) : e;
		}
		return v(o) ? r ? /* @__PURE__ */ Bt(o) : /* @__PURE__ */ Rt(o) : o;
	}
}, yt = class extends vt {
	constructor(e = !1) {
		super(!1, e);
	}
	set(e, t, n, r) {
		let i = e[t], a = d(e) && te(t);
		if (!this._isShallow) {
			let e = /* @__PURE__ */ Ut(i);
			if (!/* @__PURE__ */ k(n) && !/* @__PURE__ */ Ut(n) && (i = /* @__PURE__ */ A(i), n = /* @__PURE__ */ A(n)), !a && /* @__PURE__ */ M(i) && !/* @__PURE__ */ M(n)) return e || (i.value = n), !0;
		}
		let o = a ? Number(t) < e.length : u(e, t), s = Reflect.set(e, t, n, /* @__PURE__ */ M(e) ? e : r);
		return e === /* @__PURE__ */ A(r) && s && (o ? le(n, i) && it(e, "set", t, n, i) : it(e, "add", t, n)), s;
	}
	deleteProperty(e, t) {
		let n = u(e, t), r = e[t], i = Reflect.deleteProperty(e, t);
		return i && n && it(e, "delete", t, void 0, r), i;
	}
	has(e, t) {
		let n = Reflect.has(e, t);
		return (!_(t) || !gt.has(t)) && O(e, "has", t), n;
	}
	ownKeys(e) {
		return O(e, "iterate", d(e) ? "length" : tt), Reflect.ownKeys(e);
	}
}, bt = class extends vt {
	constructor(e = !1) {
		super(!0, e);
	}
	set(e, t) {
		return !0;
	}
	deleteProperty(e, t) {
		return !0;
	}
}, xt = /* @__PURE__ */ new yt(), St = /* @__PURE__ */ new bt(), Ct = /* @__PURE__ */ new yt(!0), wt = (e) => e, Tt = (e) => Reflect.getPrototypeOf(e);
function Et(e, t, n) {
	return function(...r) {
		let i = this.__v_raw, a = /* @__PURE__ */ A(i), o = f(a), c = e === "entries" || e === Symbol.iterator && o, l = e === "keys" && o, u = i[e](...r), d = n ? wt : t ? Kt : j;
		return !t && O(a, "iterate", l ? nt : tt), s(Object.create(u), { next() {
			let { value: e, done: t } = u.next();
			return t ? {
				value: e,
				done: t
			} : {
				value: c ? [d(e[0]), d(e[1])] : d(e),
				done: t
			};
		} });
	};
}
function Dt(e) {
	return function(...t) {
		return e === "delete" ? !1 : e === "clear" ? void 0 : this;
	};
}
function Ot(e, t) {
	let n = {
		get(n) {
			let r = this.__v_raw, i = /* @__PURE__ */ A(r), a = /* @__PURE__ */ A(n);
			e || (le(n, a) && O(i, "get", n), O(i, "get", a));
			let { has: o } = Tt(i), s = t ? wt : e ? Kt : j;
			if (o.call(i, n)) return s(r.get(n));
			if (o.call(i, a)) return s(r.get(a));
			r !== i && r.get(n);
		},
		get size() {
			let t = this.__v_raw;
			return !e && O(/* @__PURE__ */ A(t), "iterate", tt), t.size;
		},
		has(t) {
			let n = this.__v_raw, r = /* @__PURE__ */ A(n), i = /* @__PURE__ */ A(t);
			return e || (le(t, i) && O(r, "has", t), O(r, "has", i)), t === i ? n.has(t) : n.has(t) || n.has(i);
		},
		forEach(n, r) {
			let i = this, a = i.__v_raw, o = /* @__PURE__ */ A(a), s = t ? wt : e ? Kt : j;
			return !e && O(o, "iterate", tt), a.forEach((e, t) => n.call(r, s(e), s(t), i));
		}
	};
	return s(n, e ? {
		add: Dt("add"),
		set: Dt("set"),
		delete: Dt("delete"),
		clear: Dt("clear")
	} : {
		add(e) {
			let n = /* @__PURE__ */ A(this), r = Tt(n), i = /* @__PURE__ */ A(e), a = !t && !/* @__PURE__ */ k(e) && !/* @__PURE__ */ Ut(e) ? i : e;
			return r.has.call(n, a) || le(e, a) && r.has.call(n, e) || le(i, a) && r.has.call(n, i) || (n.add(a), it(n, "add", a, a)), this;
		},
		set(e, n) {
			!t && !/* @__PURE__ */ k(n) && !/* @__PURE__ */ Ut(n) && (n = /* @__PURE__ */ A(n));
			let r = /* @__PURE__ */ A(this), { has: i, get: a } = Tt(r), o = i.call(r, e);
			o || (e = /* @__PURE__ */ A(e), o = i.call(r, e));
			let s = a.call(r, e);
			return r.set(e, n), o ? le(n, s) && it(r, "set", e, n, s) : it(r, "add", e, n), this;
		},
		delete(e) {
			let t = /* @__PURE__ */ A(this), { has: n, get: r } = Tt(t), i = n.call(t, e);
			i || (e = /* @__PURE__ */ A(e), i = n.call(t, e));
			let a = r ? r.call(t, e) : void 0, o = t.delete(e);
			return i && it(t, "delete", e, void 0, a), o;
		},
		clear() {
			let e = /* @__PURE__ */ A(this), t = e.size !== 0, n = e.clear();
			return t && it(e, "clear", void 0, void 0, void 0), n;
		}
	}), [
		"keys",
		"values",
		"entries",
		Symbol.iterator
	].forEach((r) => {
		n[r] = Et(r, e, t);
	}), n;
}
function kt(e, t) {
	let n = Ot(e, t);
	return (t, r, i) => r === "__v_isReactive" ? !e : r === "__v_isReadonly" ? e : r === "__v_raw" ? t : Reflect.get(u(n, r) && r in t ? n : t, r, i);
}
var At = { get: /* @__PURE__ */ kt(!1, !1) }, jt = { get: /* @__PURE__ */ kt(!1, !0) }, Mt = { get: /* @__PURE__ */ kt(!0, !1) }, Nt = /* @__PURE__ */ new WeakMap(), Pt = /* @__PURE__ */ new WeakMap(), Ft = /* @__PURE__ */ new WeakMap(), It = /* @__PURE__ */ new WeakMap();
function Lt(e) {
	switch (e) {
		case "Object":
		case "Array": return 1;
		case "Map":
		case "Set":
		case "WeakMap":
		case "WeakSet": return 2;
		default: return 0;
	}
}
// @__NO_SIDE_EFFECTS__
function Rt(e) {
	return /* @__PURE__ */ Ut(e) ? e : Vt(e, !1, xt, At, Nt);
}
// @__NO_SIDE_EFFECTS__
function zt(e) {
	return Vt(e, !1, Ct, jt, Pt);
}
// @__NO_SIDE_EFFECTS__
function Bt(e) {
	return Vt(e, !0, St, Mt, Ft);
}
function Vt(e, t, n, r, i) {
	if (!v(e) || e.__v_raw && !(t && e.__v_isReactive) || e.__v_skip || !Object.isExtensible(e)) return e;
	let a = i.get(e);
	if (a) return a;
	let o = Lt(ee(e));
	if (o === 0) return e;
	let s = new Proxy(e, o === 2 ? r : n);
	return i.set(e, s), s;
}
// @__NO_SIDE_EFFECTS__
function Ht(e) {
	return /* @__PURE__ */ Ut(e) ? /* @__PURE__ */ Ht(e.__v_raw) : !!(e && e.__v_isReactive);
}
// @__NO_SIDE_EFFECTS__
function Ut(e) {
	return !!(e && e.__v_isReadonly);
}
// @__NO_SIDE_EFFECTS__
function k(e) {
	return !!(e && e.__v_isShallow);
}
// @__NO_SIDE_EFFECTS__
function Wt(e) {
	return e ? !!e.__v_raw : !1;
}
// @__NO_SIDE_EFFECTS__
function A(e) {
	let t = e && e.__v_raw;
	return t ? /* @__PURE__ */ A(t) : e;
}
function Gt(e) {
	return !u(e, "__v_skip") && Object.isExtensible(e) && de(e, "__v_skip", !0), e;
}
var j = (e) => v(e) ? /* @__PURE__ */ Rt(e) : e, Kt = (e) => v(e) ? /* @__PURE__ */ Bt(e) : e;
// @__NO_SIDE_EFFECTS__
function M(e) {
	return e ? e.__v_isRef === !0 : !1;
}
// @__NO_SIDE_EFFECTS__
function N(e) {
	return qt(e, !1);
}
function qt(e, t) {
	return /* @__PURE__ */ M(e) ? e : new Jt(e, t);
}
var Jt = class {
	constructor(e, t) {
		this.dep = new Qe(), this.__v_isRef = !0, this.__v_isShallow = !1, this._rawValue = t ? e : /* @__PURE__ */ A(e), this._value = t ? e : j(e), this.__v_isShallow = t;
	}
	get value() {
		return this.dep.track(), this._value;
	}
	set value(e) {
		let t = this._rawValue, n = this.__v_isShallow || /* @__PURE__ */ k(e) || /* @__PURE__ */ Ut(e);
		e = n ? e : /* @__PURE__ */ A(e), le(e, t) && (this._rawValue = e, this._value = n ? e : j(e), this.dep.trigger());
	}
};
function P(e) {
	return /* @__PURE__ */ M(e) ? e.value : e;
}
var Yt = {
	get: (e, t, n) => t === "__v_raw" ? e : P(Reflect.get(e, t, n)),
	set: (e, t, n, r) => {
		let i = e[t];
		return /* @__PURE__ */ M(i) && !/* @__PURE__ */ M(n) ? (i.value = n, !0) : Reflect.set(e, t, n, r);
	}
};
function Xt(e) {
	return /* @__PURE__ */ Ht(e) ? e : new Proxy(e, Yt);
}
var Zt = class {
	constructor(e, t, n) {
		this.fn = e, this.setter = t, this._value = void 0, this.dep = new Qe(this), this.__v_isRef = !0, this.deps = void 0, this.depsTail = void 0, this.flags = 16, this.globalVersion = Xe - 1, this.next = void 0, this.effect = this, this.__v_isReadonly = !t, this.isSSR = n;
	}
	notify() {
		if (this.flags |= 16, !(this.flags & 8) && D !== this) return Ie(this, !0), !0;
	}
	get value() {
		let e = this.dep.track();
		return He(this), e && (e.version = this.dep.version), this._value;
	}
	set value(e) {
		this.setter && this.setter(e);
	}
};
// @__NO_SIDE_EFFECTS__
function Qt(e, t, n = !1) {
	let r, i;
	return h(e) ? r = e : (r = e.get, i = e.set), new Zt(r, i, n);
}
var $t = {}, en = /* @__PURE__ */ new WeakMap(), tn = void 0;
function nn(e, t = !1, n = tn) {
	if (n) {
		let t = en.get(n);
		t || en.set(n, t = []), t.push(e);
	}
}
function rn(e, n, i = t) {
	let { immediate: a, deep: o, once: s, scheduler: l, augmentJob: u, call: f } = i, p = (e) => o ? e : /* @__PURE__ */ k(e) || o === !1 || o === 0 ? an(e, 1) : an(e), m, g, _, v, y = !1, b = !1;
	if (/* @__PURE__ */ M(e) ? (g = () => e.value, y = /* @__PURE__ */ k(e)) : /* @__PURE__ */ Ht(e) ? (g = () => p(e), y = !0) : d(e) ? (b = !0, y = e.some((e) => /* @__PURE__ */ Ht(e) || /* @__PURE__ */ k(e)), g = () => e.map((e) => {
		if (/* @__PURE__ */ M(e)) return e.value;
		if (/* @__PURE__ */ Ht(e)) return p(e);
		if (h(e)) return f ? f(e, 2) : e();
	})) : g = h(e) ? n ? f ? () => f(e, 2) : e : () => {
		if (_) {
			qe();
			try {
				_();
			} finally {
				Je();
			}
		}
		let t = tn;
		tn = m;
		try {
			return f ? f(e, 3, [v]) : e(v);
		} finally {
			tn = t;
		}
	} : r, n && o) {
		let e = g, t = o === !0 ? Infinity : o;
		g = () => an(e(), t);
	}
	let x = ke(), ee = () => {
		m.stop(), x && x.active && c(x.effects, m);
	};
	if (s && n) {
		let e = n;
		n = (...t) => {
			let n = e(...t);
			return ee(), n;
		};
	}
	let S = b ? Array(e.length).fill($t) : $t, te = (e) => {
		if (!(!(m.flags & 1) || !m.dirty && !e)) {
			if (n) {
				let t = m.run();
				if (e || o || y || (b ? t.some((e, t) => le(e, S[t])) : le(t, S))) {
					_ && _();
					let e = tn;
					tn = m;
					try {
						let e = [
							t,
							S === $t ? void 0 : b && S[0] === $t ? [] : S,
							v
						];
						S = t, f ? f(n, 3, e) : n(...e);
					} finally {
						tn = e;
					}
				}
			} else m.run();
		}
	};
	return u && u(te), m = new Me(g), m.scheduler = l ? () => l(te, !1) : te, v = (e) => nn(e, !1, m), _ = m.onStop = () => {
		let e = en.get(m);
		if (e) {
			if (f) f(e, 4);
			else for (let t of e) t();
			en.delete(m);
		}
	}, n ? a ? te(!0) : S = m.run() : l ? l(te.bind(null, !0), !0) : m.run(), ee.pause = m.pause.bind(m), ee.resume = m.resume.bind(m), ee.stop = ee, ee;
}
function an(e, t = Infinity, n) {
	if (t <= 0 || !v(e) || e.__v_skip || (n = n || /* @__PURE__ */ new Map(), (n.get(e) || 0) >= t)) return e;
	if (n.set(e, t), t--, /* @__PURE__ */ M(e)) an(e.value, t, n);
	else if (d(e)) for (let r = 0; r < e.length; r++) an(e[r], t, n);
	else if (p(e) || f(e)) e.forEach((e) => {
		an(e, t, n);
	});
	else if (S(e)) {
		for (let r in e) an(e[r], t, n);
		for (let r of Object.getOwnPropertySymbols(e)) Object.prototype.propertyIsEnumerable.call(e, r) && an(e[r], t, n);
	}
	return e;
}
//#endregion
//#region ../../node_modules/@vue/runtime-core/dist/runtime-core.esm-bundler.js
function on(e, t, n, r) {
	try {
		return r ? e(...r) : e();
	} catch (e) {
		cn(e, t, n);
	}
}
function sn(e, t, n, r) {
	if (h(e)) {
		let i = on(e, t, n, r);
		return i && y(i) && i.catch((e) => {
			cn(e, t, n);
		}), i;
	}
	if (d(e)) {
		let i = [];
		for (let a = 0; a < e.length; a++) i.push(sn(e[a], t, n, r));
		return i;
	}
}
function cn(e, n, r, i = !0) {
	let a = n ? n.vnode : null, { errorHandler: o, throwUnhandledErrorInProduction: s } = n && n.appContext.config || t;
	if (n) {
		let t = n.parent, i = n.proxy, a = `https://vuejs.org/error-reference/#runtime-${r}`;
		for (; t;) {
			let n = t.ec;
			if (n) {
				for (let t = 0; t < n.length; t++) if (n[t](e, i, a) === !1) return;
			}
			t = t.parent;
		}
		if (o) {
			qe(), on(o, null, 10, [
				e,
				i,
				a
			]), Je();
			return;
		}
	}
	ln(e, r, a, i, s);
}
function ln(e, t, n, r = !0, i = !1) {
	if (i) throw e;
	console.error(e);
}
var F = [], un = -1, dn = [], fn = null, pn = 0, mn = /* @__PURE__ */ Promise.resolve(), hn = null;
function gn(e) {
	let t = hn || mn;
	return e ? t.then(this ? e.bind(this) : e) : t;
}
function _n(e) {
	let t = un + 1, n = F.length;
	for (; t < n;) {
		let r = t + n >>> 1, i = F[r], a = Cn(i);
		a < e || a === e && i.flags & 2 ? t = r + 1 : n = r;
	}
	return t;
}
function vn(e) {
	if (!(e.flags & 1)) {
		let t = Cn(e), n = F[F.length - 1];
		!n || !(e.flags & 2) && t >= Cn(n) ? F.push(e) : F.splice(_n(t), 0, e), e.flags |= 1, yn();
	}
}
function yn() {
	hn || (hn = mn.then(wn));
}
function bn(e) {
	if (!d(e)) fn && e.id === -1 ? fn.splice(pn + 1, 0, e) : e.flags & 1 || (dn.push(e), e.flags |= 1);
	else for (let t = 0; t < e.length; t++) dn.push(e[t]);
	yn();
}
function xn(e, t, n = un + 1) {
	for (; n < F.length; n++) {
		let t = F[n];
		if (t && t.flags & 2) {
			if (e && t.id !== e.uid) continue;
			F.splice(n, 1), n--, t.flags & 4 && (t.flags &= -2), t(), t.flags & 4 || (t.flags &= -2);
		}
	}
}
function Sn(e) {
	if (dn.length) {
		let e = [...new Set(dn)].sort((e, t) => Cn(e) - Cn(t));
		if (dn.length = 0, fn) {
			for (let t = 0; t < e.length; t++) fn.push(e[t]);
			return;
		}
		for (fn = e, pn = 0; pn < fn.length; pn++) {
			let e = fn[pn];
			e.flags & 4 && (e.flags &= -2), e.flags & 8 || e(), e.flags &= -2;
		}
		fn = null, pn = 0;
	}
}
var Cn = (e) => e.id == null ? e.flags & 2 ? -1 : Infinity : e.id;
function wn(e) {
	try {
		for (un = 0; un < F.length; un++) {
			let e = F[un];
			e && !(e.flags & 8) && (e.flags & 4 && (e.flags &= -2), on(e, e.i, e.i ? 15 : 14), e.flags & 4 || (e.flags &= -2));
		}
	} finally {
		for (; un < F.length; un++) {
			let e = F[un];
			e && (e.flags &= -2);
		}
		un = -1, F.length = 0, Sn(e), hn = null, (F.length || dn.length) && wn(e);
	}
}
var I = null, Tn = null;
function En(e) {
	let t = I;
	return I = e, Tn = e && e.type.__scopeId || null, t;
}
function L(e, t = I, n) {
	if (!t || e._n) return e;
	let r = (...n) => {
		r._d && ii(-1);
		let i = En(t), a = ti.length, o;
		try {
			o = e(...n);
		} finally {
			for (let e = ti.length; e > a; e--) ni();
			En(i), r._d && ii(1);
		}
		return o;
	};
	return r._n = !0, r._c = !0, r._d = !0, r;
}
function Dn(e, n) {
	if (I === null) return e;
	let r = Fi(I), i = e.dirs || (e.dirs = []);
	for (let e = 0; e < n.length; e++) {
		let [a, o, s, c = t] = n[e];
		a && (h(a) && (a = {
			mounted: a,
			updated: a
		}), a.deep && an(o), i.push({
			dir: a,
			instance: r,
			value: o,
			oldValue: void 0,
			arg: s,
			modifiers: c
		}));
	}
	return e;
}
function On(e, t, n, r) {
	let i = e.dirs, a = t && t.dirs;
	for (let o = 0; o < i.length; o++) {
		let s = i[o];
		a && (s.oldValue = a[o].value);
		let c = s.dir[r];
		c && (qe(), sn(c, n, 8, [
			e.el,
			s,
			e,
			t
		]), Je());
	}
}
function kn(e, t, n = !1) {
	let r = Si();
	if (r || dr) {
		let i = dr ? dr._context.provides : r ? r.parent == null || r.ce ? r.vnode.appContext && r.vnode.appContext.provides : r.parent.provides : void 0;
		if (i && e in i) return i[e];
		if (arguments.length > 1) return n && h(t) ? t.call(r && r.proxy) : t;
	}
}
var An = /* @__PURE__ */ Symbol.for("v-scx"), jn = () => kn(An);
function Mn(e, t, n) {
	return Nn(e, t, n);
}
function Nn(e, n, i = t) {
	let { immediate: a, deep: o, flush: c, once: l } = i, u = s({}, i), d = n && a || !n && c !== "post", f;
	if (Oi) {
		if (c === "sync") {
			let e = jn();
			f = e.__watcherHandles || (e.__watcherHandles = []);
		} else if (!d) {
			let e = () => {};
			return e.stop = r, e.resume = r, e.pause = r, e;
		}
	}
	let p = xi;
	u.call = (e, t, n) => sn(e, p, t, n);
	let m = !1;
	c === "post" ? u.scheduler = (e) => {
		B(e, p && p.suspense);
	} : c !== "sync" && (m = !0, u.scheduler = (e, t) => {
		t ? e() : vn(e);
	}), u.augmentJob = (e) => {
		n && (e.flags |= 4), m && (e.flags |= 2, p && (e.id = p.uid, e.i = p));
	};
	let h = rn(e, n, u);
	return Oi && (f ? f.push(h) : d && h()), h;
}
var Pn = /* @__PURE__ */ Symbol("_vte"), Fn = (e) => e.__isTeleport, In = /* @__PURE__ */ Symbol("_leaveCb");
function Ln(e) {
	let t = e[0];
	if (e.length > 1) {
		for (let n of e) if (n.type !== $r) {
			t = n;
			break;
		}
	}
	return t;
}
function Rn(e) {
	if (!qn(e)) return Fn(e.type) && e.children ? Ln(e.children) : e;
	if (e.component) return e.component.subTree;
	let { shapeFlag: t, children: n } = e;
	if (n) {
		if (t & 16) return n[0];
		if (t & 32 && h(n.default)) return n.default();
	}
}
function zn(e, t) {
	if (e.shapeFlag & 6 && e.component) {
		e.transition = t;
		let n = e.component.subTree;
		zn(Fn(n.type) && Rn(n) || n, t);
	} else e.shapeFlag & 128 ? (e.ssContent.transition = t.clone(e.ssContent), e.ssFallback.transition = t.clone(e.ssFallback)) : e.transition = t;
}
// @__NO_SIDE_EFFECTS__
function R(e, t) {
	return h(e) ? /* @__PURE__ */ s({ name: e.name }, t, { setup: e }) : e;
}
function Bn() {
	let e = Si();
	return e ? (e.appContext.config.idPrefix || "v") + "-" + e.ids[0] + e.ids[1]++ : "";
}
function Vn(e) {
	e.ids = [
		e.ids[0] + e.ids[2]++ + "-",
		0,
		0
	];
}
function Hn(e, t) {
	let n;
	return !!((n = Object.getOwnPropertyDescriptor(e, t)) && !n.configurable);
}
var Un = /* @__PURE__ */ new WeakMap();
function Wn(e, n, r, a, o = !1) {
	if (d(e)) {
		e.forEach((e, t) => Wn(e, n && (d(n) ? n[t] : n), r, a, o));
		return;
	}
	if (Kn(a) && !o) {
		a.shapeFlag & 512 && a.type.__asyncResolved && a.component.subTree.component && Wn(e, n, r, a.component.subTree);
		return;
	}
	let s = a.shapeFlag & 4 ? Fi(a.component) : a.el, l = o ? null : s, { i: f, r: p } = e, m = n && n.r, _ = f.refs === t ? f.refs = {} : f.refs, v = f.setupState, y = /* @__PURE__ */ A(v), b = v === t ? i : (e) => !Hn(_, e) && u(y, e), x = (e, t) => !(t && Hn(_, t));
	if (m != null && m !== p) {
		if (Gn(n), g(m)) _[m] = null, b(m) && (v[m] = null);
		else if (/* @__PURE__ */ M(m)) {
			let e = n;
			x(m, e.k) && (m.value = null), e.k && (_[e.k] = null);
		}
	}
	if (h(p)) on(p, f, 12, [l, _]);
	else {
		let t = g(p), n = /* @__PURE__ */ M(p);
		if (t || n) {
			let i = () => {
				if (e.f) {
					let n = t ? b(p) ? v[p] : _[p] : x(p) || !e.k ? p.value : _[e.k];
					if (o) d(n) && c(n, s);
					else if (d(n)) n.includes(s) || n.push(s);
					else if (t) _[p] = [s], b(p) && (v[p] = _[p]);
					else {
						let t = [s];
						x(p, e.k) && (p.value = t), e.k && (_[e.k] = t);
					}
				} else t ? (_[p] = l, b(p) && (v[p] = l)) : n && (x(p, e.k) && (p.value = l), e.k && (_[e.k] = l));
			};
			if (l) {
				let t = () => {
					i(), Un.delete(e);
				};
				t.id = -1, Un.set(e, t), B(t, r);
			} else Gn(e), i();
		}
	}
}
function Gn(e) {
	let t = Un.get(e);
	t && (t.flags |= 8, Un.delete(e));
}
me().requestIdleCallback, me().cancelIdleCallback;
var Kn = (e) => !!e.type.__asyncLoader, qn = (e) => e.type.__isKeepAlive;
function Jn(e, t, n = xi, r = !1) {
	if (n) {
		let i = n[e] || (n[e] = []), a = t.__weh || (t.__weh = (...r) => {
			qe();
			let i = Ti(n), a = sn(t, n, e, r);
			return i(), Je(), a;
		});
		return r ? i.unshift(a) : i.push(a), a;
	}
}
function Yn(e, t = xi) {
	Jn("ec", e, t);
}
var Xn = "components";
function Zn(e, t) {
	return er(Xn, e, !0, t) || e;
}
var Qn = /* @__PURE__ */ Symbol.for("v-ndc");
function $n(e) {
	return g(e) ? er(Xn, e, !1) || e : e || Qn;
}
function er(e, t, n = !0, r = !1) {
	let i = I || xi;
	if (i) {
		let n = i.type;
		if (e === Xn) {
			let e = Ii(n, !1);
			if (e && (e === t || e === C(t) || e === se(C(t)))) return n;
		}
		let a = tr(i[e] || n[e], t) || tr(i.appContext[e], t);
		return !a && r ? n : a;
	}
}
function tr(e, t) {
	return e && (e[t] || e[C(t)] || e[se(C(t))]);
}
function z(e, t, n, r) {
	let i, a = n && n[r], o = d(e);
	if (o || g(e)) {
		let n = o && /* @__PURE__ */ Ht(e), r = !1, s = !1;
		n && (r = !/* @__PURE__ */ k(e), s = /* @__PURE__ */ Ut(e), e = ot(e)), i = Array(e.length);
		for (let n = 0, o = e.length; n < o; n++) i[n] = t(r ? s ? Kt(j(e[n])) : j(e[n]) : e[n], n, void 0, a && a[n]);
	} else if (typeof e == "number") {
		i = Array(e);
		for (let n = 0; n < e; n++) i[n] = t(n + 1, n, void 0, a && a[n]);
	} else if (v(e)) {
		if (e[Symbol.iterator]) i = Array.from(e, (e, n) => t(e, n, void 0, a && a[n]));
		else {
			let n = Object.keys(e);
			i = Array(n.length);
			for (let r = 0, o = n.length; r < o; r++) {
				let o = n[r];
				i[r] = t(e[o], o, r, a && a[r]);
			}
		}
	} else i = [];
	return n && (n[r] = i), i;
}
function nr(e, t, n, r, i, a) {
	if (n == null && (n = {}), I.ce || I.parent && Kn(I.parent) && I.parent.ce) {
		let e = a != null && n.key == null ? s({}, n, { key: a }) : n, i = Object.keys(e).length > 0;
		return t !== "default" && (e.name = t), U(), G(V, null, [q("slot", e, r && r())], i ? -2 : 64);
	}
	let o = e[t];
	o && o._c && (o._d = !1);
	let c = ti.length;
	U();
	let l;
	try {
		let i = o && rr(o(n)), s = n.key || a || i && i.key;
		l = G(V, { key: (s && !_(s) ? s : `_${t}`) + (!i && r ? "_fb" : "") }, i || (r ? r() : []), i && e._ === 1 ? 64 : -2);
	} catch (e) {
		for (let e = ti.length; e > c; e--) ni();
		throw e;
	} finally {
		o && o._c && (o._d = !0);
	}
	return !i && l.scopeId && (l.slotScopeIds = [l.scopeId + "-s"]), l;
}
function rr(e) {
	return e.some((e) => !oi(e) || !(e.type === $r || e.type === V && !rr(e.children))) ? e : null;
}
var ir = (e) => e ? Di(e) ? Fi(e) : ir(e.parent) : null, ar = /* @__PURE__ */ s(/* @__PURE__ */ Object.create(null), {
	$: (e) => e,
	$el: (e) => e.vnode.el,
	$data: (e) => e.data,
	$props: (e) => e.props,
	$attrs: (e) => e.attrs,
	$slots: (e) => e.slots,
	$refs: (e) => e.refs,
	$parent: (e) => ir(e.parent),
	$root: (e) => ir(e.root),
	$host: (e) => e.ce,
	$emit: (e) => e.emit,
	$options: (e) => e.type,
	$forceUpdate: (e) => e.f || (e.f = () => {
		vn(e.update);
	}),
	$nextTick: (e) => e.n || (e.n = gn.bind(e.proxy)),
	$watch: (e) => r
}), or = (e, n) => e !== t && !e.__isScriptSetup && u(e, n), sr = {
	get({ _: e }, n) {
		if (n === "__v_skip") return !0;
		let { ctx: r, setupState: i, data: a, props: o, accessCache: s, type: c, appContext: l } = e;
		if (n[0] !== "$") {
			let e = s[n];
			if (e !== void 0) switch (e) {
				case 1: return i[n];
				case 2: return a[n];
				case 4: return r[n];
				case 3: return o[n];
			}
			else if (or(i, n)) return s[n] = 1, i[n];
			else if (u(o, n)) return s[n] = 3, o[n];
			else if (r !== t && u(r, n)) return s[n] = 4, r[n];
			else s[n] = 0;
		}
		let d = ar[n], f, p;
		if (d) return n === "$attrs" && O(e.attrs, "get", ""), d(e);
		if ((f = c.__cssModules) && (f = f[n])) return f;
		if (r !== t && u(r, n)) return s[n] = 4, r[n];
		if (p = l.config.globalProperties, u(p, n)) return p[n];
	},
	set({ _: e }, t, n) {
		let { data: r, setupState: i, ctx: a } = e;
		return or(i, t) ? (i[t] = n, !0) : u(e.props, t) || t[0] === "$" && t.slice(1) in e ? !1 : (a[t] = n, !0);
	},
	has({ _: { data: e, setupState: t, accessCache: n, ctx: r, appContext: i, props: a, type: o } }, s) {
		let c;
		return !!(n[s] || or(t, s) || u(a, s) || u(r, s) || u(ar, s) || u(i.config.globalProperties, s) || (c = o.__cssModules) && c[s]);
	},
	defineProperty(e, t, n) {
		return n.get == null ? u(n, "value") && this.set(e, t, n.value, null) : e._.accessCache[t] = 0, Reflect.defineProperty(e, t, n);
	}
};
function cr() {
	return {
		app: null,
		config: {
			isNativeTag: i,
			performance: !1,
			globalProperties: {},
			optionMergeStrategies: {},
			errorHandler: void 0,
			warnHandler: void 0,
			compilerOptions: {}
		},
		mixins: [],
		components: {},
		directives: {},
		provides: /* @__PURE__ */ Object.create(null),
		optionsCache: /* @__PURE__ */ new WeakMap(),
		propsCache: /* @__PURE__ */ new WeakMap(),
		emitsCache: /* @__PURE__ */ new WeakMap()
	};
}
var lr = 0;
function ur(e, t) {
	return function(n, r = null) {
		h(n) || (n = s({}, n)), r != null && !v(r) && (r = null);
		let i = cr(), a = /* @__PURE__ */ new WeakSet(), o = [], c = !1, l = i.app = {
			_uid: lr++,
			_component: n,
			_props: r,
			_container: null,
			_context: i,
			_instance: null,
			version: Bi,
			get config() {
				return i.config;
			},
			set config(e) {},
			use(e, ...t) {
				return a.has(e) || (e && h(e.install) ? (a.add(e), e.install(l, ...t)) : h(e) && (a.add(e), e(l, ...t))), l;
			},
			mixin(e) {
				return l;
			},
			component(e, t) {
				return t ? (i.components[e] = t, l) : i.components[e];
			},
			directive(e, t) {
				return t ? (i.directives[e] = t, l) : i.directives[e];
			},
			mount(a, o, s) {
				if (!c) {
					let u = l._ceVNode || q(n, r);
					return u.appContext = i, s === !0 ? s = "svg" : s === !1 && (s = void 0), o && t ? t(u, a) : e(u, a, s), c = !0, l._container = a, a.__vue_app__ = l, Fi(u.component);
				}
			},
			onUnmount(e) {
				o.push(e);
			},
			unmount() {
				c && (sn(o, l._instance, 16), e(null, l._container), delete l._container.__vue_app__);
			},
			provide(e, t) {
				return i.provides[e] = t, l;
			},
			runWithContext(e) {
				let t = dr;
				dr = l;
				try {
					return e();
				} finally {
					dr = t;
				}
			}
		};
		return l;
	};
}
var dr = null, fr = (e, t) => t === "modelValue" || t === "model-value" ? e.modelModifiers : e[`${t}Modifiers`] || e[`${C(t)}Modifiers`] || e[`${oe(t)}Modifiers`];
function pr(e, n, ...r) {
	if (e.isUnmounted) return;
	let i = e.vnode.props || t, a = r, o = n.startsWith("update:"), s = o && fr(i, n.slice(7));
	s && (s.trim && (a = r.map((e) => g(e) ? e.trim() : e)), s.number && (a = r.map(fe)));
	let c, l = i[c = ce(n)] || i[c = ce(C(n))];
	!l && o && (l = i[c = ce(oe(n))]), l && sn(l, e, 6, a);
	let u = i[c + "Once"];
	if (u) {
		if (!e.emitted) e.emitted = {};
		else if (e.emitted[c]) return;
		e.emitted[c] = !0, sn(u, e, 6, a);
	}
}
function mr(e, t, n = !1) {
	let r = t.emitsCache, i = r.get(e);
	if (i !== void 0) return i;
	let a = e.emits, o = {};
	return a ? (d(a) ? a.forEach((e) => o[e] = null) : s(o, a), v(e) && r.set(e, o), o) : (v(e) && r.set(e, null), null);
}
function hr(e, t) {
	return !e || !a(t) ? !1 : (t = t.slice(2), t = t === "Once" ? t : t.replace(/Once$/, ""), u(e, t[0].toLowerCase() + t.slice(1)) || u(e, oe(t)) || u(e, t));
}
function gr(e) {
	let { type: t, vnode: n, proxy: r, withProxy: i, propsOptions: [a], slots: s, attrs: c, emit: l, render: u, renderCache: d, props: f, data: p, setupState: m, ctx: h, inheritAttrs: g } = e, _ = En(e), v, y;
	try {
		if (n.shapeFlag & 4) {
			let e = i || r, t = e;
			v = pi(u.call(t, e, d, f, m, p, h)), y = c;
		} else {
			let e = t;
			v = pi(e.length > 1 ? e(f, {
				attrs: c,
				slots: s,
				emit: l
			}) : e(f, null)), y = t.props ? c : _r(c);
		}
	} catch (t) {
		ti.length = 0, cn(t, e, 1), v = q($r);
	}
	let b = v;
	if (y && g !== !1) {
		let e = Object.keys(y), { shapeFlag: t } = b;
		e.length && t & 7 && (a && e.some(o) && (y = vr(y, a)), b = fi(b, y, !1, !0));
	}
	return n.dirs && (b = fi(b, null, !1, !0), b.dirs = b.dirs ? b.dirs.concat(n.dirs) : n.dirs), n.transition && zn(Fn(b.type) && Rn(b) || b, n.transition), v = b, En(_), v;
}
var _r = (e) => {
	let t;
	for (let n in e) (n === "class" || n === "style" || a(n)) && ((t || (t = {}))[n] = e[n]);
	return t;
}, vr = (e, t) => {
	let n = {};
	for (let r in e) (!o(r) || !(r.slice(9) in t)) && (n[r] = e[r]);
	return n;
};
function yr(e, t, n) {
	let { props: r, children: i, component: a } = e, { props: o, children: s, patchFlag: c } = t, l = a.emitsOptions;
	if (t.dirs || t.transition) return !0;
	if (n && c >= 0) {
		if (c & 1024) return !0;
		if (c & 16) return r ? br(r, o, l) : !!o;
		if (c & 8) {
			let e = t.dynamicProps;
			for (let t = 0; t < e.length; t++) {
				let n = e[t];
				if (xr(o, r, n) && !hr(l, n)) return !0;
			}
		}
	} else return (i || s) && (!s || !s.$stable) ? !0 : r === o ? !1 : r ? !o || br(r, o, l) : !!o;
	return !1;
}
function br(e, t, n) {
	let r = Object.keys(t);
	if (r.length !== Object.keys(e).length) return !0;
	for (let i = 0; i < r.length; i++) {
		let a = r[i];
		if (xr(t, e, a) && !hr(n, a)) return !0;
	}
	return !1;
}
function xr(e, t, n) {
	let r = e[n], i = t[n];
	return n === "style" && v(r) && v(i) ? !we(r, i) : r !== i;
}
function Sr({ vnode: e, parent: t, suspense: n }, r) {
	for (; t;) {
		let n = t.subTree;
		if (n.suspense && n.suspense.activeBranch === e && (n.suspense.vnode.el = n.el = r, e = n), n === e) (e = t.vnode).el = r, t = t.parent;
		else break;
	}
	n && n.activeBranch === e && (n.vnode.el = r);
}
var Cr = {}, wr = () => Object.create(Cr), Tr = (e) => Object.getPrototypeOf(e) === Cr;
function Er(e, t, n, r = !1) {
	let i = {}, a = wr();
	e.propsDefaults = /* @__PURE__ */ Object.create(null), Or(e, t, i, a);
	for (let t in e.propsOptions[0]) t in i || (i[t] = void 0);
	e.props = n ? r ? i : /* @__PURE__ */ zt(i) : e.type.props ? i : a, e.attrs = a;
}
function Dr(e, t, n, r) {
	let { props: i, attrs: a, vnode: { patchFlag: o } } = e, s = /* @__PURE__ */ A(i), [c] = e.propsOptions, l = !1;
	if ((r || o > 0) && !(o & 16)) {
		if (o & 8) {
			let n = e.vnode.dynamicProps;
			for (let r = 0; r < n.length; r++) {
				let o = n[r];
				if (hr(e.emitsOptions, o)) continue;
				let d = t[o];
				if (c) {
					if (u(a, o)) d !== a[o] && (a[o] = d, l = !0);
					else {
						let t = C(o);
						i[t] = kr(c, s, t, d, e, !1);
					}
				} else d !== a[o] && (a[o] = d, l = !0);
			}
		}
	} else {
		Or(e, t, i, a) && (l = !0);
		let r;
		for (let a in s) (!t || !u(t, a) && ((r = oe(a)) === a || !u(t, r))) && (c ? n && (n[a] !== void 0 || n[r] !== void 0) && (i[a] = kr(c, s, a, void 0, e, !0)) : delete i[a]);
		if (a !== s) for (let e in a) (!t || !u(t, e)) && (delete a[e], l = !0);
	}
	l && it(e.attrs, "set", "");
}
function Or(e, n, r, i) {
	let [a, o] = e.propsOptions, s = !1, c;
	if (n) for (let t in n) {
		if (ne(t)) continue;
		let l = n[t], d;
		a && u(a, d = C(t)) ? !o || !o.includes(d) ? r[d] = l : (c || (c = {}))[d] = l : hr(e.emitsOptions, t) || (!(t in i) || l !== i[t]) && (i[t] = l, s = !0);
	}
	if (o) {
		let n = /* @__PURE__ */ A(r), i = c || t;
		for (let t = 0; t < o.length; t++) {
			let s = o[t];
			r[s] = kr(a, n, s, i[s], e, !u(i, s));
		}
	}
	return s;
}
function kr(e, t, n, r, i, a) {
	let o = e[n];
	if (o != null) {
		let e = u(o, "default");
		if (e && r === void 0) {
			let e = o.default;
			if (o.type !== Function && !o.skipFactory && h(e)) {
				let { propsDefaults: a } = i;
				if (n in a) r = a[n];
				else {
					let o = Ti(i);
					r = a[n] = e.call(null, t), o();
				}
			} else r = e;
			i.ce && i.ce._setProp(n, r);
		}
		o[0] && (a && !e ? r = !1 : o[1] && (r === "" || r === oe(n)) && (r = !0));
	}
	return r;
}
function Ar(e, r, i = !1) {
	let a = r.propsCache, o = a.get(e);
	if (o) return o;
	let c = e.props, l = {}, f = [];
	if (!c) return v(e) && a.set(e, n), n;
	if (d(c)) for (let e = 0; e < c.length; e++) {
		let n = C(c[e]);
		jr(n) && (l[n] = t);
	}
	else if (c) for (let e in c) {
		let t = C(e);
		if (jr(t)) {
			let n = c[e], r = l[t] = d(n) || h(n) ? { type: n } : s({}, n), i = r.type, a = !1, o = !0;
			if (d(i)) for (let e = 0; e < i.length; ++e) {
				let t = i[e], n = h(t) && t.name;
				if (n === "Boolean") {
					a = !0;
					break;
				}
				n === "String" && (o = !1);
			}
			else a = h(i) && i.name === "Boolean";
			r[0] = a, r[1] = o, (a || u(r, "default")) && f.push(t);
		}
	}
	let p = [l, f];
	return v(e) && a.set(e, p), p;
}
function jr(e) {
	return e[0] !== "$" && !ne(e);
}
var Mr = (e) => e === "_" || e === "_ctx" || e === "$stable", Nr = (e) => d(e) ? e.map(pi) : [pi(e)], Pr = (e, t, n) => {
	if (t._n) return t;
	let r = L((...e) => Nr(t(...e)), n);
	return r._c = !1, r;
}, Fr = (e, t, n) => {
	let r = e._ctx;
	for (let n in e) {
		if (Mr(n)) continue;
		let i = e[n];
		if (h(i)) t[n] = Pr(n, i, r);
		else if (i != null) {
			let e = Nr(i);
			t[n] = () => e;
		}
	}
}, Ir = (e, t) => {
	let n = Nr(t);
	e.slots.default = () => n;
}, Lr = (e, t, n) => {
	for (let r in t) (n || !Mr(r)) && (e[r] = t[r]);
}, Rr = (e, t, n) => {
	let r = e.slots = wr();
	if (e.vnode.shapeFlag & 32) {
		let e = t._;
		e ? (Lr(r, t, n), n && de(r, "_", e, !0)) : Fr(t, r);
	} else t && Ir(e, t);
}, zr = (e, n, r) => {
	let { vnode: i, slots: a } = e, o = !0, s = t;
	if (i.shapeFlag & 32) {
		let e = n._;
		e ? r && e === 1 ? o = !1 : Lr(a, n, r) : (o = !n.$stable, Fr(n, a)), s = n;
	} else n && (Ir(e, n), s = { default: 1 });
	if (o) for (let e in a) !Mr(e) && s[e] == null && delete a[e];
}, B = Zr;
function Br(e) {
	return Vr(e);
}
function Vr(e, i) {
	let a = me();
	a.__VUE__ = !0;
	let { insert: o, remove: s, patchProp: c, createElement: l, createText: u, createComment: d, setText: f, setElementText: p, parentNode: m, nextSibling: h, setScopeId: g = r, insertStaticContent: _ } = e, v = (e, t, n, r = null, i = null, a = null, o = void 0, s = null, c = !!t.dynamicChildren) => {
		if (e === t) return;
		e && !si(e, t) && (r = we(e), w(e, i, a, !0), e = null), t.patchFlag === -2 && (c = !1, t.dynamicChildren = null);
		let { type: l, ref: u, shapeFlag: d } = t;
		switch (l) {
			case Qr:
				y(e, t, n, r);
				break;
			case $r:
				b(e, t, n, r);
				break;
			case ei:
				e == null && x(t, n, r, o);
				break;
			case V:
				ce(e, t, n, r, i, a, o, s, c);
				break;
			default: d & 1 ? te(e, t, n, r, i, a, o, s, c) : d & 6 ? le(e, t, n, r, i, a, o, s, c) : (d & 64 || d & 128) && l.process(e, t, n, r, i, a, o, s, c, Ee);
		}
		u != null && i ? Wn(u, e && e.ref, a, t || e, !t) : u == null && e && e.ref != null && Wn(e.ref, null, a, e, !0);
	}, y = (e, t, n, r) => {
		if (e == null) o(t.el = u(t.children), n, r);
		else {
			let n = t.el = e.el;
			t.children !== e.children && f(n, t.children);
		}
	}, b = (e, t, n, r) => {
		e == null ? o(t.el = d(t.children || ""), n, r) : t.el = e.el;
	}, x = (e, t, n, r) => {
		[e.el, e.anchor] = _(e.children, t, n, r, e.el, e.anchor);
	}, ee = ({ el: e, anchor: t }, n, r) => {
		let i;
		for (; e && e !== t;) i = h(e), o(e, n, r), e = i;
		o(t, n, r);
	}, S = ({ el: e, anchor: t }) => {
		let n;
		for (; e && e !== t;) n = h(e), s(e), e = n;
		s(t);
	}, te = (e, t, n, r, i, a, o, s, c) => {
		if (t.type === "svg" ? o = "svg" : t.type === "math" && (o = "mathml"), e == null) re(t, n, r, i, a, o, s, c);
		else {
			let n = e.el && e.el._isVueCE ? e.el : null;
			try {
				n && n._beginPatch(), ae(e, t, i, a, o, s, c);
			} finally {
				n && n._endPatch();
			}
		}
	}, re = (e, t, n, r, i, a, s, u) => {
		let d, f, { props: m, shapeFlag: h, transition: g, dirs: _ } = e;
		if (d = e.el = l(e.type, a, m && m.is, m), h & 8 ? p(d, e.children) : h & 16 && C(e.children, d, null, r, i, Hr(e, a), s, u), _ && On(e, null, r, "created"), ie(d, e, e.scopeId, s, r), m) {
			for (let e in m) e !== "value" && !ne(e) && c(d, e, null, m[e], a, r);
			"value" in m && c(d, "value", null, m.value, a), (f = m.onVnodeBeforeMount) && _i(f, r, e);
		}
		_ && On(e, null, r, "beforeMount");
		let v = Wr(i, g);
		v && g.beforeEnter(d), o(d, t, n), ((f = m && m.onVnodeMounted) || v || _) && B(() => {
			try {
				f && _i(f, r, e), v && g.enter(d), _ && On(e, null, r, "mounted");
			} finally {}
		}, i);
	}, ie = (e, t, n, r, i) => {
		if (n && g(e, n), r) for (let t = 0; t < r.length; t++) g(e, r[t]);
		if (i) {
			let n = i.subTree;
			if (t === n || Xr(n.type) && (n.ssContent === t || n.ssFallback === t)) {
				let t = i.vnode;
				ie(e, t, t.scopeId, t.slotScopeIds, i.parent);
			}
		}
	}, C = (e, t, n, r, i, a, o, s, c = 0) => {
		for (let l = c; l < e.length; l++) {
			let c = e[l] = s ? mi(e[l]) : pi(e[l]);
			v(null, c, t, n, r, i, a, o, s);
		}
	}, ae = (e, n, r, i, a, o, s) => {
		let l = n.el = e.el, { patchFlag: u, dynamicChildren: d, dirs: f } = n;
		u |= e.patchFlag & 16;
		let m = e.props || t, h = n.props || t, g;
		if (r && Ur(r, !1), (g = h.onVnodeBeforeUpdate) && _i(g, r, n, e), f && On(n, e, r, "beforeUpdate"), r && Ur(r, !0), d && (!e.dynamicChildren || e.dynamicChildren.length !== d.length) && (u = 0, s = !1, d = null), (m.innerHTML && h.innerHTML == null || m.textContent && h.textContent == null) && p(l, ""), d ? oe(e.dynamicChildren, d, l, r, i, Hr(n, a), o) : s || ge(e, n, l, null, r, i, Hr(n, a), o, !1), u > 0) {
			if (u & 16) se(l, m, h, r, a);
			else if (u & 2 && m.class !== h.class && c(l, "class", null, h.class, a), u & 4 && c(l, "style", m.style, h.style, a), u & 8) {
				let e = n.dynamicProps;
				for (let t = 0; t < e.length; t++) {
					let n = e[t], i = m[n], o = h[n];
					(o !== i || n === "value") && c(l, n, i, o, a, r);
				}
			}
			u & 1 && e.children !== n.children && p(l, n.children);
		} else !s && d == null && se(l, m, h, r, a);
		((g = h.onVnodeUpdated) || f) && B(() => {
			g && _i(g, r, n, e), f && On(n, e, r, "updated");
		}, i);
	}, oe = (e, t, n, r, i, a, o) => {
		for (let s = 0; s < t.length; s++) {
			let c = e[s], l = t[s], u = c.el && (c.type === V || !si(c, l) || c.shapeFlag & 198) ? m(c.el) : n;
			v(c, l, u, null, r, i, a, o, !0);
		}
	}, se = (e, n, r, i, a) => {
		if (n !== r) {
			if (n !== t) for (let t in n) !ne(t) && !(t in r) && c(e, t, n[t], null, a, i);
			for (let t in r) {
				if (ne(t)) continue;
				let o = r[t], s = n[t];
				o !== s && t !== "value" && c(e, t, s, o, a, i);
			}
			"value" in r && c(e, "value", n.value, r.value, a);
		}
	}, ce = (e, t, n, r, i, a, s, c, l) => {
		let d = t.el = e ? e.el : u(""), f = t.anchor = e ? e.anchor : u(""), { patchFlag: p, dynamicChildren: m, slotScopeIds: h } = t;
		h && (c = c ? c.concat(h) : h), e == null ? (o(d, n, r), o(f, n, r), C(t.children || [], n, f, i, a, s, c, l)) : p > 0 && p & 64 && m && e.dynamicChildren && e.dynamicChildren.length === m.length ? (oe(e.dynamicChildren, m, n, i, a, s, c), (t.key != null || i && t === i.subTree) && Gr(e, t, !0)) : ge(e, t, n, f, i, a, s, c, l);
	}, le = (e, t, n, r, i, a, o, s, c) => {
		t.slotScopeIds = s, e == null ? t.shapeFlag & 512 ? i.ctx.activate(t, n, r, o, c) : de(t, n, r, i, a, o, c) : fe(e, t, c);
	}, de = (e, t, n, r, i, a, o) => {
		let s = e.component = bi(e, r, i);
		if (qn(e) && (s.ctx.renderer = Ee), ki(s, !1, o), s.asyncDep) {
			if (i && i.registerDep(s, pe, o), !e.el) {
				let r = s.subTree = q($r);
				b(null, r, t, n), e.placeholder = r.el;
			}
		} else pe(s, e, t, n, i, a, o);
	}, fe = (e, t, n) => {
		let r = t.component = e.component;
		if (yr(e, t, n)) {
			if (r.asyncDep && !r.asyncResolved) {
				he(r, t, n);
				return;
			}
			r.next = t, r.update();
		} else t.el = e.el, r.vnode = t;
	}, pe = (e, t, n, r, i, a, o) => {
		let s = () => {
			if (e.isMounted) {
				let { next: t, bu: n, u: r, parent: s, vnode: c } = e;
				{
					let n = qr(e);
					if (n) {
						t && (t.el = c.el, he(e, t, o)), n.asyncDep.then(() => {
							B(() => {
								e.isUnmounted || l();
							}, i);
						});
						return;
					}
				}
				let u = t, d;
				Ur(e, !1), t ? (t.el = c.el, he(e, t, o)) : t = c, n && ue(n), (d = t.props && t.props.onVnodeBeforeUpdate) && _i(d, s, t, c), Ur(e, !0);
				let f = gr(e), p = e.subTree;
				e.subTree = f, v(p, f, m(p.el), we(p), e, i, a), t.el = f.el, u === null && Sr(e, f.el), r && B(r, i), (d = t.props && t.props.onVnodeUpdated) && B(() => _i(d, s, t, c), i);
			} else {
				let o, { el: s, props: c } = t, { bm: l, m: u, parent: d, root: f, type: p } = e, m = Kn(t);
				if (Ur(e, !1), l && ue(l), !m && (o = c && c.onVnodeBeforeMount) && _i(o, d, t), Ur(e, !0), s && E) {
					let t = () => {
						e.subTree = gr(e), E(s, e.subTree, e, i, null);
					};
					m && p.__asyncHydrate ? p.__asyncHydrate(s, e, t) : t();
				} else {
					f.ce && f.ce._hasShadowRoot() && f.ce._injectChildStyle(p, e.parent ? e.parent.type : void 0);
					let o = e.subTree = gr(e);
					v(null, o, n, r, e, i, a), t.el = o.el;
				}
				if (u && B(u, i), !m && (o = c && c.onVnodeMounted)) {
					let e = t;
					B(() => _i(o, d, e), i);
				}
				(t.shapeFlag & 256 || d && Kn(d.vnode) && d.vnode.shapeFlag & 256) && e.a && B(e.a, i), e.isMounted = !0, t = n = r = null;
			}
		};
		e.scope.on();
		let c = e.effect = new Me(s);
		e.scope.off();
		let l = e.update = c.run.bind(c), u = e.job = c.runIfDirty.bind(c);
		u.i = e, u.id = e.uid, c.scheduler = () => vn(u), Ur(e, !0), l();
	}, he = (e, t, n) => {
		t.component = e;
		let r = e.vnode.props;
		e.vnode = t, e.next = null, Dr(e, t.props, r, n), zr(e, t.children, n), qe(), xn(e), Je();
	}, ge = (e, t, n, r, i, a, o, s, c = !1) => {
		let l = e && e.children, u = e ? e.shapeFlag : 0, d = t.children, { patchFlag: f, shapeFlag: m } = t;
		if (f > 0) {
			if (f & 128) {
				ve(l, d, n, r, i, a, o, s, c);
				return;
			}
			if (f & 256) {
				_e(l, d, n, r, i, a, o, s, c);
				return;
			}
		}
		m & 8 ? (u & 16 && Ce(l, i, a), d !== l && p(n, d)) : u & 16 ? m & 16 ? ve(l, d, n, r, i, a, o, s, c) : Ce(l, i, a, !0) : (u & 8 && p(n, ""), m & 16 && C(d, n, r, i, a, o, s, c));
	}, _e = (e, t, r, i, a, o, s, c, l) => {
		e = e || n, t = t || n;
		let u = e.length, d = t.length, f = Math.min(u, d), p;
		for (p = 0; p < f; p++) {
			let n = t[p] = l ? mi(t[p]) : pi(t[p]);
			v(e[p], n, r, null, a, o, s, c, l);
		}
		u > d ? Ce(e, a, o, !0, !1, f) : C(t, r, i, a, o, s, c, l, f);
	}, ve = (e, t, r, i, a, o, s, c, l) => {
		let u = 0, d = t.length, f = e.length - 1, p = d - 1;
		for (; u <= f && u <= p;) {
			let n = e[u], i = t[u] = l ? mi(t[u]) : pi(t[u]);
			if (si(n, i)) v(n, i, r, null, a, o, s, c, l);
			else break;
			u++;
		}
		for (; u <= f && u <= p;) {
			let n = e[f], i = t[p] = l ? mi(t[p]) : pi(t[p]);
			if (si(n, i)) v(n, i, r, null, a, o, s, c, l);
			else break;
			f--, p--;
		}
		if (u > f) {
			if (u <= p) {
				let e = p + 1, n = e < d ? t[e].el : i;
				for (; u <= p;) v(null, t[u] = l ? mi(t[u]) : pi(t[u]), r, n, a, o, s, c, l), u++;
			}
		} else if (u > p) for (; u <= f;) w(e[u], a, o, !0), u++;
		else {
			let m = u, h = u, g = /* @__PURE__ */ new Map();
			for (u = h; u <= p; u++) {
				let e = t[u] = l ? mi(t[u]) : pi(t[u]);
				e.key != null && g.set(e.key, u);
			}
			let _, y = 0, b = p - h + 1, x = !1, ee = 0, S = Array(b);
			for (u = 0; u < b; u++) S[u] = 0;
			for (u = m; u <= f; u++) {
				let n = e[u];
				if (y >= b) {
					w(n, a, o, !0);
					continue;
				}
				let i;
				if (n.key != null) i = g.get(n.key);
				else for (_ = h; _ <= p; _++) if (S[_ - h] === 0 && si(n, t[_])) {
					i = _;
					break;
				}
				i === void 0 ? w(n, a, o, !0) : (S[i - h] = u + 1, i >= ee ? ee = i : x = !0, v(n, t[i], r, null, a, o, s, c, l), y++);
			}
			let te = x ? Kr(S) : n;
			for (_ = te.length - 1, u = b - 1; u >= 0; u--) {
				let e = h + u, n = t[e], f = t[e + 1], p = e + 1 < d ? f.el || Yr(f) : i;
				S[u] === 0 ? v(null, n, r, p, a, o, s, c, l) : x && (_ < 0 || u !== te[_] ? ye(n, r, p, 2) : _--);
			}
		}
	}, ye = (e, t, n, r, i = null) => {
		let { el: a, type: c, transition: l, children: u, shapeFlag: d } = e;
		if (d & 6) {
			ye(e.component.subTree, t, n, r);
			return;
		}
		if (d & 128) {
			e.suspense.move(t, n, r);
			return;
		}
		if (d & 64) {
			c.move(e, t, n, Ee);
			return;
		}
		if (c === V) {
			o(a, t, n);
			for (let e = 0; e < u.length; e++) ye(u[e], t, n, r);
			o(e.anchor, t, n);
			return;
		}
		if (c === ei) {
			ee(e, t, n);
			return;
		}
		if (r !== 2 && d & 1 && l) {
			if (r === 0) l.persisted && !a[In] ? o(a, t, n) : (l.beforeEnter(a), o(a, t, n), B(() => l.enter(a), i));
			else {
				let { leave: r, delayLeave: i, afterLeave: c } = l, u = () => {
					e.ctx.isUnmounted ? s(a) : o(a, t, n);
				}, d = () => {
					let e = a._isLeaving || !!a[In];
					a._isLeaving && a[In](!0), l.persisted && !e ? u() : r(a, () => {
						u(), c && c();
					});
				};
				i ? i(a, u, d) : d();
			}
		} else o(a, t, n);
	}, w = (e, t, n, r = !1, i = !1) => {
		let { type: a, props: o, ref: s, children: c, dynamicChildren: l, shapeFlag: u, patchFlag: d, dirs: f, cacheIndex: p, memo: m } = e;
		if (d === -2 && (i = !1), s != null && (qe(), Wn(s, null, n, e, !0), Je()), p != null && (t.renderCache[p] = void 0), u & 256) {
			t.ctx.deactivate(e);
			return;
		}
		let h = u & 1 && f, g = !Kn(e), _;
		if (g && (_ = o && o.onVnodeBeforeUnmount) && _i(_, t, e), u & 6) Se(e.component, n, r);
		else {
			if (u & 128) {
				e.suspense.unmount(n, r);
				return;
			}
			h && On(e, null, t, "beforeUnmount"), u & 64 ? e.type.remove(e, t, n, Ee, r) : l && !l.hasOnce && (a !== V || d > 0 && d & 64) ? Ce(l, t, n, !1, !0) : (a === V && d & 384 || !i && u & 16) && Ce(c, t, n), r && be(e);
		}
		let v = m != null && p == null;
		(g && (_ = o && o.onVnodeUnmounted) || h || v) && B(() => {
			_ && _i(_, t, e), h && On(e, null, t, "unmounted"), v && (e.el = null);
		}, n);
	}, be = (e) => {
		let { type: t, el: n, anchor: r, transition: i } = e;
		if (t === V) {
			xe(n, r);
			return;
		}
		if (t === ei) {
			S(e);
			return;
		}
		let a = () => {
			s(n), i && !i.persisted && i.afterLeave && i.afterLeave();
		};
		if (e.shapeFlag & 1 && i && !i.persisted) {
			let { leave: t, delayLeave: r } = i, o = () => t(n, a);
			r ? r(e.el, a, o) : o();
		} else a();
	}, xe = (e, t) => {
		let n;
		for (; e !== t;) n = h(e), s(e), e = n;
		s(t);
	}, Se = (e, t, n) => {
		let { bum: r, scope: i, job: a, subTree: o, um: s, m: c, a: l } = e;
		Jr(c), Jr(l), r && ue(r), i.stop(), a && (a.flags |= 8, w(o, e, t, n)), s && B(s, t), B(() => {
			e.isUnmounted = !0;
		}, t);
	}, Ce = (e, t, n, r = !1, i = !1, a = 0) => {
		for (let o = a; o < e.length; o++) w(e[o], t, n, r, i);
	}, we = (e) => {
		if (e.shapeFlag & 6) return we(e.component.subTree);
		if (e.shapeFlag & 128) return e.suspense.next();
		let t = h(e.anchor || e.el), n = t && t[Pn];
		return n ? h(n) : t;
	}, Te = !1, T = (e, t, n) => {
		let r;
		e == null ? t._vnode && (w(t._vnode, null, null, !0), r = t._vnode.component) : v(t._vnode || null, e, t, null, null, null, n), t._vnode = e, Te || (Te = !0, xn(r), Sn(), Te = !1);
	}, Ee = {
		p: v,
		um: w,
		m: ye,
		r: be,
		mt: de,
		mc: C,
		pc: ge,
		pbc: oe,
		n: we,
		o: e
	}, De, E;
	return i && ([De, E] = i(Ee)), {
		render: T,
		hydrate: De,
		createApp: ur(T, De)
	};
}
function Hr({ type: e, props: t }, n) {
	return n === "svg" && e === "foreignObject" || n === "mathml" && e === "annotation-xml" && t && t.encoding && t.encoding.includes("html") ? void 0 : n;
}
function Ur({ effect: e, job: t }, n) {
	n ? (e.flags |= 32, t.flags |= 4) : (e.flags &= -33, t.flags &= -5);
}
function Wr(e, t) {
	return (!e || e && !e.pendingBranch) && t && !t.persisted;
}
function Gr(e, t, n = !1) {
	let r = e.children, i = t.children;
	if (d(r) && d(i)) for (let e = 0; e < r.length; e++) {
		let t = r[e], a = i[e];
		a.shapeFlag & 1 && !a.dynamicChildren && ((a.patchFlag <= 0 || a.patchFlag === 32) && (a = i[e] = mi(i[e]), a.el = t.el), !n && a.patchFlag !== -2 && Gr(t, a)), a.type === Qr && (a.patchFlag === -1 && (a = i[e] = mi(a)), a.el = t.el), a.type === $r && !a.el && (a.el = t.el);
	}
}
function Kr(e) {
	let t = e.slice(), n = [0], r, i, a, o, s, c = e.length;
	for (r = 0; r < c; r++) {
		let c = e[r];
		if (c !== 0) {
			if (i = n[n.length - 1], e[i] < c) {
				t[r] = i, n.push(r);
				continue;
			}
			for (a = 0, o = n.length - 1; a < o;) s = a + o >> 1, e[n[s]] < c ? a = s + 1 : o = s;
			c < e[n[a]] && (a > 0 && (t[r] = n[a - 1]), n[a] = r);
		}
	}
	for (a = n.length, o = n[a - 1]; a-- > 0;) n[a] = o, o = t[o];
	return n;
}
function qr(e) {
	let t = e.subTree.component;
	if (t) return t.asyncDep && !t.asyncResolved ? t : qr(t);
}
function Jr(e) {
	if (e) for (let t = 0; t < e.length; t++) e[t].flags |= 8;
}
function Yr(e) {
	if (e.placeholder) return e.placeholder;
	let t = e.component;
	return t ? Yr(t.subTree) : null;
}
var Xr = (e) => e.__isSuspense;
function Zr(e, t) {
	t && t.pendingBranch ? d(e) ? t.effects.push(...e) : t.effects.push(e) : bn(e);
}
var V = /* @__PURE__ */ Symbol.for("v-fgt"), Qr = /* @__PURE__ */ Symbol.for("v-txt"), $r = /* @__PURE__ */ Symbol.for("v-cmt"), ei = /* @__PURE__ */ Symbol.for("v-stc"), ti = [], H = null;
function U(e = !1) {
	ti.push(H = e ? null : []);
}
function ni() {
	ti.pop(), H = ti[ti.length - 1] || null;
}
var ri = 1;
function ii(e, t = !1) {
	ri += e, e < 0 && H && t && (H.hasOnce = !0);
}
function ai(e) {
	return e.dynamicChildren = ri > 0 ? H || n : null, ni(), ri > 0 && H && H.push(e), e;
}
function W(e, t, n, r, i, a) {
	return ai(K(e, t, n, r, i, a, !0));
}
function G(e, t, n, r, i) {
	return ai(q(e, t, n, r, i, !0));
}
function oi(e) {
	return e ? e.__v_isVNode === !0 : !1;
}
function si(e, t) {
	return e.type === t.type && e.key === t.key;
}
var ci = ({ key: e }) => e == null ? null : e, li = ({ ref: e, ref_key: t, ref_for: n }) => (typeof e == "number" && (e = "" + e), e == null ? null : g(e) || /* @__PURE__ */ M(e) || h(e) ? {
	i: I,
	r: e,
	k: t,
	f: !!n
} : e);
function K(e, t = null, n = null, r = 0, i = null, a = e === V ? 0 : 1, o = !1, s = !1) {
	let c = {
		__v_isVNode: !0,
		__v_skip: !0,
		type: e,
		props: t,
		key: t && ci(t),
		ref: t && li(t),
		scopeId: Tn,
		slotScopeIds: null,
		children: n,
		component: null,
		suspense: null,
		ssContent: null,
		ssFallback: null,
		dirs: null,
		transition: null,
		el: null,
		anchor: null,
		target: null,
		targetStart: null,
		targetAnchor: null,
		staticCount: 0,
		shapeFlag: a,
		patchFlag: r,
		dynamicProps: i,
		dynamicChildren: null,
		appContext: null,
		ctx: I
	};
	return s ? (hi(c, n), a & 128 && e.normalize(c)) : n && (c.shapeFlag |= g(n) ? 8 : 16), ri > 0 && !o && H && (c.patchFlag > 0 || a & 6) && c.patchFlag !== 32 && H.push(c), c;
}
var q = ui;
function ui(e, t = null, n = null, r = 0, i = null, a = !1) {
	if ((!e || e === Qn) && (e = $r), oi(e)) {
		let r = fi(e, t, !0);
		return n && hi(r, n), ri > 0 && !a && H && (r.shapeFlag & 6 ? H[H.indexOf(e)] = r : H.push(r)), r.patchFlag = -2, r;
	}
	if (Li(e) && (e = e.__vccOpts), t) {
		t = di(t);
		let { class: e, style: n } = t;
		e && !g(e) && (t.class = w(e)), v(n) && (/* @__PURE__ */ Wt(n) && !d(n) && (n = s({}, n)), t.style = he(n));
	}
	let o = g(e) ? 1 : Xr(e) ? 128 : Fn(e) ? 64 : v(e) ? 4 : h(e) ? 2 : 0;
	return K(e, t, n, r, i, o, a, !0);
}
function di(e) {
	return e ? /* @__PURE__ */ Wt(e) || Tr(e) ? s({}, e) : e : null;
}
function fi(e, t, n = !1, r = !1) {
	let { props: i, ref: a, patchFlag: o, children: s, transition: c } = e, l = t ? gi(i || {}, t) : i, u = {
		__v_isVNode: !0,
		__v_skip: !0,
		type: e.type,
		props: l,
		key: l && ci(l),
		ref: t && t.ref ? n && a ? d(a) ? a.concat(li(t)) : [a, li(t)] : li(t) : a,
		scopeId: e.scopeId,
		slotScopeIds: e.slotScopeIds,
		children: s,
		target: e.target,
		targetStart: e.targetStart,
		targetAnchor: e.targetAnchor,
		staticCount: e.staticCount,
		shapeFlag: e.shapeFlag,
		patchFlag: t && e.type !== V ? o === -1 ? 16 : o | 16 : o,
		dynamicProps: e.dynamicProps,
		dynamicChildren: e.dynamicChildren,
		appContext: e.appContext,
		dirs: e.dirs,
		transition: c,
		component: e.component,
		suspense: e.suspense,
		ssContent: e.ssContent && fi(e.ssContent),
		ssFallback: e.ssFallback && fi(e.ssFallback),
		placeholder: e.placeholder,
		el: e.el,
		anchor: e.anchor,
		ctx: e.ctx,
		ce: e.ce
	};
	return c && r && zn(u, c.clone(u)), u;
}
function J(e = " ", t = 0) {
	return q(Qr, null, e, t);
}
function Y(e = "", t = !1) {
	return t ? (U(), G($r, null, e)) : q($r, null, e);
}
function pi(e) {
	return e == null || typeof e == "boolean" ? q($r) : d(e) ? q(V, null, e.slice()) : oi(e) ? mi(e) : q(Qr, null, String(e));
}
function mi(e) {
	return e.el === null && e.patchFlag !== -1 || e.memo ? e : fi(e);
}
function hi(e, t) {
	let n = 0, { shapeFlag: r } = e;
	if (t == null) t = null;
	else if (d(t)) n = 16;
	else if (typeof t == "object") {
		if (r & 65) {
			let n = t.default;
			n && (n._c && (n._d = !1), hi(e, n()), n._c && (n._d = !0));
			return;
		}
		{
			n = 32;
			let r = t._;
			!r && !Tr(t) ? t._ctx = I : r === 3 && I && (I.slots._ === 1 ? t._ = 1 : (t._ = 2, e.patchFlag |= 1024));
		}
	} else if (h(t)) {
		if (r & 65) {
			hi(e, { default: t });
			return;
		}
		t = {
			default: t,
			_ctx: I
		}, n = 32;
	} else t = String(t), r & 64 ? (n = 16, t = [J(t)]) : n = 8;
	e.children = t, e.shapeFlag |= n;
}
function gi(...e) {
	let t = {};
	for (let n = 0; n < e.length; n++) {
		let r = e[n];
		for (let e in r) if (e === "class") t.class !== r.class && (t.class = w([t.class, r.class]));
		else if (e === "style") t.style = he([t.style, r.style]);
		else if (a(e)) {
			let n = t[e], i = r[e];
			i && n !== i && !(d(n) && n.includes(i)) ? t[e] = n ? [].concat(n, i) : i : i == null && n == null && !o(e) && (t[e] = i);
		} else e !== "" && (t[e] = r[e]);
	}
	return t;
}
function _i(e, t, n, r = null) {
	sn(e, t, 7, [n, r]);
}
var vi = cr(), yi = 0;
function bi(e, n, r) {
	let i = e.type, a = (n ? n.appContext : e.appContext) || vi, o = {
		uid: yi++,
		vnode: e,
		type: i,
		parent: n,
		appContext: a,
		root: null,
		next: null,
		subTree: null,
		effect: null,
		update: null,
		job: null,
		scope: new Oe(!0),
		render: null,
		proxy: null,
		exposed: null,
		exposeProxy: null,
		withProxy: null,
		provides: n ? n.provides : Object.create(a.provides),
		ids: n ? n.ids : [
			"",
			0,
			0
		],
		accessCache: null,
		renderCache: [],
		components: null,
		directives: null,
		propsOptions: Ar(i, a),
		emitsOptions: mr(i, a),
		emit: null,
		emitted: null,
		propsDefaults: t,
		inheritAttrs: i.inheritAttrs,
		ctx: t,
		data: t,
		props: t,
		attrs: t,
		slots: t,
		refs: t,
		setupState: t,
		setupContext: null,
		suspense: r,
		suspenseId: r ? r.pendingId : 0,
		asyncDep: null,
		asyncResolved: !1,
		isMounted: !1,
		isUnmounted: !1,
		isDeactivated: !1,
		bc: null,
		c: null,
		bm: null,
		m: null,
		bu: null,
		u: null,
		um: null,
		bum: null,
		da: null,
		a: null,
		rtg: null,
		rtc: null,
		ec: null,
		sp: null
	};
	return o.ctx = { _: o }, o.root = n ? n.root : o, o.emit = pr.bind(null, o), e.ce && e.ce(o), o;
}
var xi = null, Si = () => xi || I, Ci, wi;
{
	let e = me(), t = (t, n) => {
		let r;
		return (r = e[t]) || (r = e[t] = []), r.push(n), (e) => {
			r.length > 1 ? r.forEach((t) => t(e)) : r[0](e);
		};
	};
	Ci = t("__VUE_INSTANCE_SETTERS__", (e) => xi = e), wi = t("__VUE_SSR_SETTERS__", (e) => Oi = e);
}
var Ti = (e) => {
	let t = xi;
	return Ci(e), e.scope.on(), () => {
		e.scope.off(), Ci(t);
	};
}, Ei = () => {
	xi && xi.scope.off(), Ci(null);
};
function Di(e) {
	return e.vnode.shapeFlag & 4;
}
var Oi = !1;
function ki(e, t = !1, n = !1) {
	t && wi(t);
	let { props: r, children: i } = e.vnode, a = Di(e);
	Er(e, r, a, t), Rr(e, i, n || t);
	let o = a ? Ai(e, t) : void 0;
	return t && wi(!1), o;
}
function Ai(e, t) {
	let n = e.type;
	e.accessCache = /* @__PURE__ */ Object.create(null), e.proxy = new Proxy(e.ctx, sr);
	let { setup: r } = n;
	if (r) {
		qe();
		let n = e.setupContext = r.length > 1 ? Pi(e) : null, i = Ti(e), a = on(r, e, 0, [e.props, n]), o = y(a);
		if (Je(), i(), (o || e.sp) && !Kn(e) && Vn(e), o) {
			if (a.then(Ei, Ei), t) return a.then((n) => {
				wi(!0);
				try {
					ji(e, n, t);
				} finally {
					wi(!1);
				}
			}).catch((t) => {
				cn(t, e, 0);
			});
			e.asyncDep = a;
		} else ji(e, a, t);
	} else Mi(e, t);
}
function ji(e, t, n) {
	h(t) ? e.type.__ssrInlineRender ? e.ssrRender = t : e.render = t : v(t) && (e.setupState = Xt(t)), Mi(e, n);
}
function Mi(e, t, n) {
	let i = e.type;
	e.render || (e.render = i.render || r);
}
var Ni = { get(e, t) {
	return O(e, "get", ""), e[t];
} };
function Pi(e) {
	return {
		attrs: new Proxy(e.attrs, Ni),
		slots: e.slots,
		emit: e.emit,
		expose: (t) => {
			e.exposed = t || {};
		}
	};
}
function Fi(e) {
	return e.exposed ? e.exposeProxy || (e.exposeProxy = new Proxy(Xt(Gt(e.exposed)), {
		get(t, n) {
			if (n in t) return t[n];
			if (n in ar) return ar[n](e);
		},
		has(e, t) {
			return t in e || t in ar;
		}
	})) : e.proxy;
}
function Ii(e, t = !0) {
	return h(e) ? e.displayName || e.name : e.name || t && e.__name;
}
function Li(e) {
	return h(e) && "__vccOpts" in e;
}
var Ri = (e, t) => /* @__PURE__ */ Qt(e, t, Oi);
function zi(e, t, n) {
	try {
		ii(-1);
		let r = arguments.length;
		return r === 2 ? v(t) && !d(t) ? oi(t) ? q(e, null, [t]) : q(e, t) : q(e, null, t) : (r > 3 ? n = Array.prototype.slice.call(arguments, 2) : r === 3 && oi(n) && (n = [n]), q(e, t, n));
	} finally {
		ii(1);
	}
}
var Bi = "3.5.41", Vi = void 0, Hi = typeof window < "u" && window.trustedTypes;
if (Hi) try {
	Vi = /* @__PURE__ */ Hi.createPolicy("vue", { createHTML: (e) => e });
} catch {}
var Ui = Vi ? (e) => Vi.createHTML(e) : (e) => e, Wi = "http://www.w3.org/2000/svg", Gi = "http://www.w3.org/1998/Math/MathML", Ki = typeof document < "u" ? document : null, qi = Ki && /* @__PURE__ */ Ki.createElement("template"), Ji = {
	insert: (e, t, n) => {
		t.insertBefore(e, n || null);
	},
	remove: (e) => {
		let t = e.parentNode;
		t && t.removeChild(e);
	},
	createElement: (e, t, n, r) => {
		let i = t === "svg" ? Ki.createElementNS(Wi, e) : t === "mathml" ? Ki.createElementNS(Gi, e) : n ? Ki.createElement(e, { is: n }) : Ki.createElement(e);
		return e === "select" && r && r.multiple != null && i.setAttribute("multiple", r.multiple), i;
	},
	createText: (e) => Ki.createTextNode(e),
	createComment: (e) => Ki.createComment(e),
	setText: (e, t) => {
		e.nodeValue = t;
	},
	setElementText: (e, t) => {
		e.textContent = t;
	},
	parentNode: (e) => e.parentNode,
	nextSibling: (e) => e.nextSibling,
	querySelector: (e) => Ki.querySelector(e),
	setScopeId(e, t) {
		e.setAttribute(t, "");
	},
	insertStaticContent(e, t, n, r, i, a) {
		let o = n ? n.previousSibling : t.lastChild;
		if (i && (i === a || i.nextSibling)) for (; t.insertBefore(i.cloneNode(!0), n), !(i === a || !(i = i.nextSibling)););
		else {
			qi.innerHTML = Ui(r === "svg" ? `<svg>${e}</svg>` : r === "mathml" ? `<math>${e}</math>` : e);
			let i = qi.content;
			if (r === "svg" || r === "mathml") {
				let e = i.firstChild;
				for (; e.firstChild;) i.appendChild(e.firstChild);
				i.removeChild(e);
			}
			t.insertBefore(i, n);
		}
		return [o ? o.nextSibling : t.firstChild, n ? n.previousSibling : t.lastChild];
	}
}, Yi = /* @__PURE__ */ Symbol("_vtc");
function Xi(e, t, n) {
	let r = e[Yi];
	r && (t = (t ? [t, ...r] : [...r]).join(" ")), t == null ? e.removeAttribute("class") : n ? e.setAttribute("class", t) : e.className = t;
}
var Zi = /* @__PURE__ */ Symbol("_vod"), Qi = /* @__PURE__ */ Symbol("_vsh"), $i = {
	name: "show",
	beforeMount(e, { value: t }, { transition: n }) {
		e[Zi] = e.style.display === "none" ? "" : e.style.display, n && t ? n.beforeEnter(e) : ea(e, t);
	},
	mounted(e, { value: t }, { transition: n }) {
		n && t && n.enter(e);
	},
	updated(e, { value: t, oldValue: n }, { transition: r }) {
		!t != !n && (r ? t ? (r.beforeEnter(e), ea(e, !0), r.enter(e)) : r.leave(e, () => {
			ea(e, !1);
		}) : ea(e, t));
	},
	beforeUnmount(e, { value: t }) {
		ea(e, t);
	}
};
function ea(e, t) {
	e.style.display = t ? e[Zi] : "none", e[Qi] = !t;
}
var ta = /* @__PURE__ */ Symbol(""), na = /(?:^|;)\s*display\s*:/;
function ra(e, t, n) {
	let r = e.style, i = g(n), a = !1;
	if (n && !i) {
		if (t) {
			if (g(t)) for (let e of t.split(";")) {
				let t = e.slice(0, e.indexOf(":")).trim();
				n[t] == null && aa(r, t, "");
			}
			else for (let e in t) n[e] == null && aa(r, e, "");
		}
		for (let i in n) {
			i === "display" && (a = !0);
			let o = n[i];
			o == null ? aa(r, i, "") : la(e, i, !g(t) && t ? t[i] : void 0, o) || aa(r, i, o);
		}
	} else if (i) {
		if (t !== n) {
			let e = r[ta];
			e && (n += ";" + e), r.cssText = n, a = na.test(n);
		}
	} else t && e.removeAttribute("style");
	Zi in e && (e[Zi] = a ? r.display : "", e[Qi] && (r.display = "none"));
}
var ia = /\s*!important$/;
function aa(e, t, n) {
	if (d(n)) n.forEach((n) => aa(e, t, n));
	else if (n == null && (n = ""), t.startsWith("--")) e.setProperty(t, n);
	else {
		let r = ca(e, t);
		ia.test(n) ? e.setProperty(oe(r), n.replace(ia, ""), "important") : e[r] = n;
	}
}
var oa = [
	"Webkit",
	"Moz",
	"ms"
], sa = {};
function ca(e, t) {
	let n = sa[t];
	if (n) return n;
	let r = C(t);
	if (r !== "filter" && r in e) return sa[t] = r;
	r = se(r);
	for (let n = 0; n < oa.length; n++) {
		let i = oa[n] + r;
		if (i in e) return sa[t] = i;
	}
	return t;
}
function la(e, t, n, r) {
	return e.tagName === "TEXTAREA" && (t === "width" || t === "height") && g(r) && n === r;
}
var ua = "http://www.w3.org/1999/xlink";
function da(e, t, n, r, i, a = xe(t)) {
	r && t.startsWith("xlink:") ? n == null ? e.removeAttributeNS(ua, t.slice(6, t.length)) : e.setAttributeNS(ua, t, n) : n == null || a && !Se(n) ? e.removeAttribute(t) : e.setAttribute(t, a ? "" : _(n) ? String(n) : n);
}
function fa(e, t, n, r, i) {
	if (t === "innerHTML" || t === "textContent") {
		n != null && (e[t] = t === "innerHTML" ? Ui(n) : n);
		return;
	}
	let a = e.tagName;
	if (t === "value" && a !== "PROGRESS" && !a.includes("-")) {
		let r = a === "OPTION" ? e.getAttribute("value") || "" : e.value, i = n == null ? e.type === "checkbox" ? "on" : "" : String(n);
		(r !== i || !("_value" in e)) && (e.value = i), n == null && e.removeAttribute(t), e._value = n;
		return;
	}
	let o = !1;
	if (n === "" || n == null) {
		let r = typeof e[t];
		r === "boolean" ? n = Se(n) : n == null && r === "string" ? (n = "", o = !0) : r === "number" && (n = 0, o = !0);
	}
	try {
		e[t] = n;
	} catch {}
	o && e.removeAttribute(i || t);
}
function pa(e, t, n, r) {
	e.addEventListener(t, n, r);
}
function ma(e, t, n, r) {
	e.removeEventListener(t, n, r);
}
var ha = /* @__PURE__ */ Symbol("_vei");
function ga(e, t, n, r, i = null) {
	let a = e[ha] || (e[ha] = {}), o = a[t];
	if (r && o) o.value = r;
	else {
		let [n, s] = ya(t);
		r ? pa(e, n, a[t] = Ca(r, i), s) : o && (ma(e, n, o, s), a[t] = void 0);
	}
}
var _a = /(Once|Passive|Capture)$/, va = /^on:?(?:Once|Passive|Capture)$/;
function ya(e) {
	let t, n;
	for (; (n = e.match(_a)) && !va.test(e);) t || (t = {}), e = e.slice(0, e.length - n[1].length), t[n[1].toLowerCase()] = !0;
	return [e[2] === ":" ? e.slice(3) : oe(e.slice(2)), t];
}
var ba = 0, xa = /* @__PURE__ */ Promise.resolve(), Sa = () => ba || (xa.then(() => ba = 0), ba = Date.now());
function Ca(e, t) {
	let n = (e) => {
		if (!e._vts) e._vts = Date.now();
		else if (e._vts <= n.attached) return;
		let r = n.value;
		if (d(r)) {
			let n = e.stopImmediatePropagation;
			e.stopImmediatePropagation = () => {
				n.call(e), e._stopped = !0;
			};
			let i = r.slice(), a = [e];
			for (let n = 0; n < i.length && !e._stopped; n++) {
				let e = i[n];
				e && sn(e, t, 5, a);
			}
		} else sn(r, t, 5, [e]);
	};
	return n.value = e, n.attached = Sa(), n;
}
var wa = (e) => e.charCodeAt(0) === 111 && e.charCodeAt(1) === 110 && e.charCodeAt(2) > 96 && e.charCodeAt(2) < 123, Ta = (e, t, n, r, i, s) => {
	let c = i === "svg";
	t === "class" ? Xi(e, r, c) : t === "style" ? ra(e, n, r) : a(t) ? o(t) || ga(e, t, n, r, s) : (t[0] === "." ? (t = t.slice(1), !0) : t[0] === "^" ? (t = t.slice(1), !1) : Ea(e, t, r, c)) ? (fa(e, t, r), !e.tagName.includes("-") && (t === "value" || t === "checked" || t === "selected") && da(e, t, r, c, s, t !== "value")) : e._isVueCE && (Da(e, t) || e._def.__asyncLoader && (/[A-Z]/.test(t) || !g(r))) ? fa(e, C(t), r, s, t) : (t === "true-value" ? e._trueValue = r : t === "false-value" && (e._falseValue = r), da(e, t, r, c));
};
function Ea(e, t, n, r) {
	if (r) return !!(t === "innerHTML" || t === "textContent" || t in e && wa(t) && h(n));
	if (t === "spellcheck" || t === "draggable" || t === "translate" || t === "autocorrect" || t === "sandbox" && e.tagName === "IFRAME" || t === "form" || t === "list" && e.tagName === "INPUT" || t === "type" && e.tagName === "TEXTAREA") return !1;
	if (t === "width" || t === "height") {
		let t = e.tagName;
		if (t === "IMG" || t === "VIDEO" || t === "CANVAS" || t === "SOURCE") return !1;
	}
	return wa(t) && g(n) ? !1 : t in e;
}
function Da(e, t) {
	let n = e._def.props;
	if (!n) return !1;
	let r = C(t);
	return Array.isArray(n) ? n.some((e) => C(e) === r) : Object.keys(n).some((e) => C(e) === r);
}
var Oa = [
	"ctrl",
	"shift",
	"alt",
	"meta"
], ka = {
	stop: (e) => e.stopPropagation(),
	prevent: (e) => e.preventDefault(),
	self: (e) => e.target !== e.currentTarget,
	ctrl: (e) => !e.ctrlKey,
	shift: (e) => !e.shiftKey,
	alt: (e) => !e.altKey,
	meta: (e) => !e.metaKey,
	left: (e) => "button" in e && e.button !== 0,
	middle: (e) => "button" in e && e.button !== 1,
	right: (e) => "button" in e && e.button !== 2,
	exact: (e, t) => Oa.some((n) => e[`${n}Key`] && !t.includes(n))
}, Aa = (e, t) => {
	if (!e) return e;
	let n = e._withMods || (e._withMods = {}), r = t.join(".");
	return n[r] || (n[r] = ((n, ...r) => {
		for (let e = 0; e < t.length; e++) {
			let r = ka[t[e]];
			if (r && r(n, t)) return;
		}
		return e(n, ...r);
	}));
}, ja = {
	esc: "escape",
	space: " ",
	up: "arrow-up",
	left: "arrow-left",
	right: "arrow-right",
	down: "arrow-down",
	delete: "backspace"
}, Ma = (e, t) => {
	let n = e._withKeys || (e._withKeys = {}), r = t.join(".");
	return n[r] || (n[r] = ((n) => {
		if (!("key" in n)) return;
		let r = oe(n.key);
		if (t.some((e) => e === r || ja[e] === r)) return e(n);
	}));
}, Na = /* @__PURE__ */ s({ patchProp: Ta }, Ji), Pa;
function Fa() {
	return Pa || (Pa = Br(Na));
}
var Ia = ((...e) => {
	let t = Fa().createApp(...e), { mount: n } = t;
	return t.mount = (e) => {
		let r = Ra(e);
		if (!r) return;
		let i = t._component;
		!h(i) && !i.render && !i.template && (i.template = r.innerHTML), r.nodeType === 1 && (r.textContent = "");
		let a = n(r, !1, La(r));
		return r instanceof Element && (r.removeAttribute("v-cloak"), r.setAttribute("data-v-app", "")), a;
	}, t;
});
function La(e) {
	if (e instanceof SVGElement) return "svg";
	if (typeof MathMLElement == "function" && e instanceof MathMLElement) return "mathml";
}
function Ra(e) {
	return g(e) ? document.querySelector(e) : e;
}
//#endregion
//#region src/shared/config/contract.ts
var za = "data-ik-island", Ba = "data-ik-props", Va = "data-ik-state", Ha = `[${za}]`, Ua = "ik-context", Wa = "ik-messages", Ga = "ik-root", Ka = "--ik-reserve", qa = "data-ik-lazy", Ja = "data-ik-url-prefix", Ya = "islandKitHost", Xa = Symbol("ik.host"), Za = Symbol("ik.i18n"), Qa = Symbol("ik.query"), $a = Symbol("ik.emit"), eo = Symbol("ik.scrollLock"), to = Symbol("ik.media");
//#endregion
//#region src/shared/lib/url/types.ts
function no(e) {
	return typeof e == "string" ? [e] : e;
}
function ro(e) {
	let [t] = e;
	return e.length === 1 && t !== void 0 ? t : [...e];
}
//#endregion
//#region src/shared/lib/url/memory-query-port.ts
function io(e = {}) {
	let t = /* @__PURE__ */ new Map();
	for (let [n, r] of Object.entries(e)) {
		let e = no(r);
		e.length > 0 && t.set(n, [...e]);
	}
	let n = /* @__PURE__ */ new Set(), r = () => {
		let e = {};
		for (let [n, r] of t) e[n] = ro(r);
		return Object.freeze(e);
	}, i = (e, t) => e !== void 0 && e.length === t.length && e.every((e, n) => e === t[n]);
	return {
		live: !1,
		get(e) {
			var n;
			return (n = t.get(e)) == null ? void 0 : n[0];
		},
		getAll(e) {
			var n;
			return (n = t.get(e)) == null ? [] : n;
		},
		snapshot: r,
		set(e) {
			let a = !1;
			for (let [n, r] of Object.entries(e)) {
				let e = r === void 0 ? [] : no(r);
				if (e.length === 0) {
					a = t.delete(n) || a;
					continue;
				}
				i(t.get(n), e) || (t.set(n, [...e]), a = !0);
			}
			if (a) {
				let e = r();
				n.forEach((t) => t(e));
			}
		},
		subscribe(e) {
			return n.add(e), () => n.delete(e);
		}
	};
}
//#endregion
//#region src/shared/lib/dom/query-port.ts
function ao(e = {}) {
	var t, n, r, i;
	let a = (t = e.prefix) == null ? "" : t, o = (n = e.win) == null ? window : n, s = ((r = e.live) == null || r) && typeof ((i = o.history) == null ? void 0 : i.replaceState) == "function", c = () => new URLSearchParams(o.location.search), l = (e) => {
		if (!e.startsWith(a)) return null;
		let t = e.slice(a.length);
		return t === "" ? null : t;
	}, u = () => {
		let e = /* @__PURE__ */ new Map();
		for (let [t, n] of c()) {
			let r = l(t);
			if (r === null) continue;
			let i = e.get(r);
			i ? i.push(n) : e.set(r, [n]);
		}
		let t = {};
		for (let [n, r] of e) t[n] = ro(r);
		return Object.freeze(t);
	};
	if (!s) return io(u());
	let d = /* @__PURE__ */ new Set(), f = null, p = () => {
		let e = u();
		d.forEach((t) => t(e));
	};
	return {
		live: !0,
		get(e) {
			var t;
			return (t = c().get(a + e)) == null ? void 0 : t;
		},
		getAll(e) {
			return c().getAll(a + e);
		},
		snapshot: u,
		set(e, t = {}) {
			let n = c();
			for (let [t, r] of Object.entries(e)) {
				let e = a + t;
				if (n.delete(e), r !== void 0) for (let t of no(r)) n.append(e, t);
			}
			let r = n.toString(), i = `${o.location.pathname}${r ? `?${r}` : ""}${o.location.hash}`;
			t.push ? o.history.pushState(o.history.state, "", i) : o.history.replaceState(o.history.state, "", i), p();
		},
		subscribe(e) {
			if (d.add(e), f === null) {
				let e = () => p();
				o.addEventListener("popstate", e), f = () => o.removeEventListener("popstate", e);
			}
			return () => {
				d.delete(e), d.size === 0 && f !== null && (f(), f = null);
			};
		}
	};
}
//#endregion
//#region src/shared/lib/dom/emit-port.ts
function oo(e) {
	let { el: t } = e;
	return { emit(e, n) {
		e.includes(":") && console.warn(`[islandkit] event name "${e}" contains a colon; the "ik:" prefix is added automatically, so pass a bare kebab-case name.`), t.dispatchEvent(new CustomEvent("ik:" + e, {
			detail: n,
			bubbles: !0,
			composed: !0
		}));
	} };
}
//#endregion
//#region src/shared/lib/dom/scroll-lock-port.ts
var so = /* @__PURE__ */ new WeakMap();
function co(e = {}) {
	var t;
	let n = (t = e.doc) == null ? document : t;
	return {
		lock() {
			var e;
			let t = (e = so.get(n)) == null ? {
				holds: 0,
				previous: ""
			} : e;
			t.holds === 0 && (t.previous = n.body.style.overflow, n.body.style.overflow = "hidden"), t.holds += 1, so.set(n, t);
		},
		unlock() {
			let e = so.get(n);
			e !== void 0 && e.holds !== 0 && (--e.holds, e.holds === 0 && (e.previous === "" ? n.body.style.removeProperty("overflow") : n.body.style.overflow = e.previous), so.set(n, e));
		}
	};
}
//#endregion
//#region src/shared/lib/dom/media-port.ts
function lo(e = {}) {
	var t;
	let n = (t = e.win) == null ? window : t, r = /* @__PURE__ */ new Map(), i = (e) => {
		if (typeof n.matchMedia != "function") return null;
		let t = r.get(e);
		if (t !== void 0) return t;
		let i = n.matchMedia(e);
		return r.set(e, i), i;
	};
	return {
		matches: (e) => {
			var t, n;
			return (t = (n = i(e)) == null ? void 0 : n.matches) != null && t;
		},
		watch(e, t) {
			let n = i(e);
			if (n === null) return () => void 0;
			let r = (e) => {
				t(e.matches);
			};
			return n.addEventListener("change", r), () => {
				n.removeEventListener("change", r);
			};
		}
	};
}
//#endregion
//#region src/shared/lib/i18n/messages.ts
var uo = {
	en: {
		"kit.clear": "Clear",
		"kit.selectAll": "Select all",
		"kit.selected": "Selected: {count}",
		"kit.loading": "Loading",
		"kit.empty": "Nothing found",
		"kit.error": "Something went wrong",
		"kit.close": "Close",
		"kit.submenu": "Submenu of {name}",
		"kit.openMenu": "Open menu",
		"kit.closeMenu": "Close menu"
	},
	ru: {
		"kit.clear": "Очистить",
		"kit.selectAll": "Выбрать все",
		"kit.selected": "Выбрано: {count}",
		"kit.loading": "Загрузка",
		"kit.empty": "Ничего не найдено",
		"kit.error": "Что-то пошло не так",
		"kit.close": "Закрыть",
		"kit.submenu": "Подменю «{name}»",
		"kit.openMenu": "Открыть меню",
		"kit.closeMenu": "Закрыть меню"
	}
};
function fo(e) {
	var t, n, r;
	return (n = (r = uo[(t = e.toLowerCase().split("-")[0]) == null ? "en" : t]) == null ? uo.en : r) == null ? {} : n;
}
//#endregion
//#region src/shared/lib/i18n/create-i18n.ts
var po = /\{(\w+)\}/g;
function mo(e, t) {
	return t ? e.replace(po, (e, n) => Object.prototype.hasOwnProperty.call(t, n) ? String(t[n]) : e) : e;
}
function ho(e, t = {}) {
	let n = Object.freeze({
		...fo(e),
		...t
	}), r = (e) => n[e], i = (e, t) => {
		let n = r(e);
		return n === void 0 ? e : mo(n, t);
	}, a = new Intl.PluralRules(e);
	return {
		locale: e,
		messages: n,
		t: i,
		n: (e, t, n) => {
			var i, o;
			let s = a.select(t), c = (i = (o = r(`${e}.${s}`)) == null ? r(`${e}.other`) : o) == null ? r(e) : i;
			return c === void 0 ? e : mo(c, {
				count: t,
				...n
			});
		}
	};
}
//#endregion
//#region src/shared/lib/i18n/use-i18n.ts
function go() {
	var e;
	return (e = kn(Za, null)) == null ? ho("en") : e;
}
//#endregion
//#region src/shared/lib/url/use-query.ts
function _o() {
	var e;
	return (e = kn(Qa, null)) == null ? io() : e;
}
//#endregion
//#region src/shared/lib/events/memory-emit-port.ts
function vo() {
	let e = [];
	return {
		emitted: e,
		emit(t, n) {
			e.push({
				name: t,
				detail: n
			});
		}
	};
}
//#endregion
//#region src/shared/lib/events/use-emit.ts
function yo() {
	var e;
	let t = (e = kn($a, null)) == null ? vo() : e;
	return (e, n) => {
		t.emit(e, n);
	};
}
//#endregion
//#region src/shared/lib/scroll-lock/memory-scroll-lock-port.ts
function bo() {
	let e = 0;
	return {
		held: () => e,
		lock() {
			e += 1;
		},
		unlock() {
			e = Math.max(0, e - 1);
		}
	};
}
//#endregion
//#region src/shared/lib/scroll-lock/use-scroll-lock.ts
function xo(e) {
	var t;
	let n = (t = kn(eo, null)) == null ? bo() : t, r = !1, i = (e) => {
		e !== r && (r = e, e ? n.lock() : n.unlock());
	};
	Mn(e, i, { immediate: !0 }), Ae(() => {
		i(!1);
	});
}
//#endregion
//#region src/shared/lib/media/memory-media-port.ts
function So(e = {}) {
	let t = new Map(Object.entries(e)), n = /* @__PURE__ */ new Map();
	return {
		matches: (e) => t.get(e) === !0,
		watch(e, t) {
			var r;
			let i = (r = n.get(e)) == null ? /* @__PURE__ */ new Set() : r;
			return i.add(t), n.set(e, i), () => {
				i.delete(t);
			};
		},
		set(e, r) {
			var i;
			if (t.get(e) !== r) {
				t.set(e, r);
				for (let t of (i = n.get(e)) == null ? [] : i) t(r);
			}
		}
	};
}
//#endregion
//#region src/shared/lib/media/use-media.ts
function Co(e) {
	var t;
	let n = (t = kn(to, null)) == null ? So() : t, r = /* @__PURE__ */ N(n.matches(e));
	return Ae(n.watch(e, (e) => {
		r.value = e;
	})), r;
}
//#endregion
//#region src/app/providers/install-context.ts
function wo(e, t) {
	var n, r, i, a, o;
	let s = ho(t.host.locale, (n = t.messages) == null ? {} : n);
	e.provide(Xa, t.host), e.provide(Za, s), e.provide(Qa, (r = t.query) == null ? io() : r), e.provide($a, (i = t.events) == null ? vo() : i), e.provide(eo, (a = t.scrollLock) == null ? bo() : a), e.provide(to, (o = t.media) == null ? So() : o);
}
//#endregion
//#region src/app/registry/types.ts
function To(e) {
	return e;
}
//#endregion
//#region src/app/registry/registry.ts
var Eo = /* @__PURE__ */ new Map();
function Do(e) {
	Eo.set(e.id, e);
}
function Oo(e) {
	e.forEach(Do);
}
function ko(e) {
	return Eo.get(e);
}
function Ao() {
	return [...Eo.keys()];
}
//#endregion
//#region src/app/mount/read-props.ts
var jo = "data-ik-", Mo = /* @__PURE__ */ new Set([
	za,
	Ba,
	Va,
	qa,
	Ja
]), No = class extends Error {
	constructor(e) {
		super(e), this.name = "IslandPropsError";
	}
};
function Po(e) {
	return e.replace(/-([a-z0-9])/g, (e, t) => t.toUpperCase());
}
function Fo(e) {
	return e === "true" ? !0 : e === "false" ? !1 : e !== "" && !Number.isNaN(Number(e)) ? Number(e) : e;
}
function Io(e, t) {
	let n = `[id="${t.replace(/["\\]/g, "\\$&")}"]`, r = e.querySelectorAll(n).length;
	r > 1 && console.warn(`[islandkit] the page has ${r} elements with id "${t}"; the first one wins, so an island may be reading a payload meant for another. Make payload ids unique per page.`);
}
function Lo(e) {
	let t = {}, n = e.getAttribute(Ba);
	if (n) {
		let i = e.ownerDocument, a = i.getElementById(n);
		if (!a) throw new No(`props element "#${n}" not found`);
		Io(i, n);
		let o;
		try {
			var r;
			o = JSON.parse((r = a.textContent) == null ? "" : r);
		} catch (e) {
			throw new No(`props element "#${n}" is not valid JSON: ${String(e)}`);
		}
		if (typeof o != "object" || !o || Array.isArray(o)) throw new No(`props element "#${n}" must contain a JSON object`);
		Object.assign(t, o);
	}
	for (let n of Array.from(e.attributes)) !n.name.startsWith(jo) || Mo.has(n.name) || (t[Po(n.name.slice(8))] = Fo(n.value));
	return t;
}
//#endregion
//#region src/app/mount/mount-island.ts
var Ro = /* @__PURE__ */ new WeakSet(), zo = /* @__PURE__ */ new Map();
async function Bo(e) {
	let t = e.component;
	return typeof t == "function" && !("render" in t) && !("setup" in t) ? (await t()).default : t;
}
function Vo(e, t, n) {
	return /* @__PURE__ */ R({
		name: "IkIslandBoundary",
		setup() {
			return Yn((e, t, r) => (n(e, r.includes("render") || r.includes("setup")), !1)), () => zi(e, t);
		}
	});
}
function Ho(e, t, n) {
	var r, i;
	let a = (r = t.getAttribute("data-ik-url-prefix")) == null ? "" : r;
	if (!(e.urlSync === !0 && n.routing.ownsUrl)) return {
		port: ao({
			prefix: a,
			live: !1
		}),
		releasePrefix: null
	};
	let o = zo.get(a);
	o !== void 0 && !o.el.isConnected && zo.delete(a);
	let s = (i = zo.get(a)) == null ? void 0 : i.island;
	return s === void 0 ? zo.set(a, {
		island: e.id,
		el: t
	}) : console.warn("[islandkit] islands \"" + s + "\" and \"" + e.id + "\" both write query parameters under prefix \"" + a + "\"; give one of them a distinct " + Ja + "."), {
		port: ao({
			prefix: a,
			live: !0
		}),
		releasePrefix: () => {
			var t;
			((t = zo.get(a)) == null ? void 0 : t.island) === e.id && zo.delete(a);
		}
	};
}
function Uo(e) {
	var t;
	return e.children.length > 0 || ((t = e.textContent) == null ? "" : t).trim() !== "";
}
function Wo(e, t) {
	if (e.kind !== "content" || Uo(t)) return;
	let n = `[islandkit] island "${e.id}" is kind:'content' but its mount point is empty. Content islands replace markup the host printed; without it the block exists only for readers who run scripts. Print the server-rendered markup inside the element, or mark the island interactive if crawlers may ignore it.`;
	console.error(n), oo({ el: t }).emit("fallback-missing", { island: e.id });
}
async function Go(e, t, n) {
	let r = typeof e == "string" ? ko(e) : e, i = typeof e == "string" ? e : e.id;
	if (!r) return console.warn(`[islandkit] no island registered as "${i}"; leaving markup untouched`), null;
	if (Ro.has(t)) return null;
	Wo(r, t), Ro.add(t);
	let a = t.innerHTML;
	t.setAttribute(Va, "pending"), t.classList.add(Ga), r.minHeight && t instanceof HTMLElement && t.style.setProperty(Ka, `${r.minHeight}px`);
	let o = null, s = null, c = !1, l = "mounting", u = null, d = () => {
		t.classList.remove(Ga), t instanceof HTMLElement && t.style.removeProperty(Ka);
	}, f = () => {
		Ro.delete(t), o == null || o();
	}, p = (e) => {
		l !== "down" && (l = "down", n.host.onError(e, { island: r.id }), queueMicrotask(() => {
			s == null || s.unmount(), t.innerHTML = a, d(), t.setAttribute(Va, "error"), f();
		}));
	}, m = (e, t) => {
		if (l === "mounting") {
			t ? u = e : n.host.onError(e, { island: r.id });
			return;
		}
		t ? p(e) : n.host.onError(e, { island: r.id });
	};
	try {
		var h, g, _, v;
		let e = {
			...Lo(t),
			...n.props
		}, i = r.parseProps ? r.parseProps(e, n.host) : e, a = await Bo(r), p = n.query;
		if (!p) {
			let e = Ho(r, t, n.host);
			p = e.port, o = e.releasePrefix;
		}
		if (s = Ia(Vo(a, i, m)), s.config.errorHandler = (e) => {
			m(e, !0);
		}, wo(s, {
			host: n.host,
			query: p,
			events: (h = n.events) == null ? oo({ el: t }) : h,
			scrollLock: (g = n.scrollLock) == null ? co() : g,
			media: (_ = n.media) == null ? lo() : _,
			...n.messages ? { messages: n.messages } : {}
		}), (v = r.setup) == null || v.call(r, s, n.host), s.mount(t), c = !0, u !== null) throw u;
		return l = "live", t.setAttribute(Va, "ready"), {
			id: r.id,
			el: t,
			unmount() {
				l !== "down" && (l = "down", s == null || s.unmount(), d(), f(), t.removeAttribute(Va));
			}
		};
	} catch (e) {
		return l = "down", c && (s == null || s.unmount(), t.innerHTML = a), d(), f(), t.setAttribute(Va, "error"), n.host.onError(e, { island: r.id }), null;
	}
}
function Ko(e) {
	return e.getAttribute(za);
}
//#endregion
//#region src/shared/lib/host/defaults.ts
function qo(e = {}) {
	var t, n, r, i, a, o, s, c, l, u, d, f, p, m, h;
	return {
		host: (t = e.host) == null ? "unknown" : t,
		locale: (n = e.locale) == null ? "en" : n,
		api: {
			baseUrl: (r = (i = e.api) == null ? void 0 : i.baseUrl) == null ? "" : r,
			...((a = e.api) == null ? void 0 : a.csrfToken) === void 0 ? {} : { csrfToken: e.api.csrfToken },
			...((o = e.api) == null ? void 0 : o.csrfHeader) === void 0 ? {} : { csrfHeader: e.api.csrfHeader }
		},
		routing: {
			origin: (s = (c = e.routing) == null ? void 0 : c.origin) == null ? "" : s,
			base: (l = (u = e.routing) == null ? void 0 : u.base) == null ? "/" : l,
			ownsUrl: (d = (f = e.routing) == null ? void 0 : f.ownsUrl) != null && d
		},
		flags: Object.freeze({ ...(p = e.flags) == null ? {} : p }),
		analytics: (m = e.analytics) == null ? { track: () => {} } : m,
		onError: (h = e.onError) == null ? ((e, t) => {
			console.error(`[islandkit] island "${t.island}" failed`, e);
		}) : h
	};
}
//#endregion
//#region src/app/mount/read-context.ts
function Jo(e, t) {
	let n = e.getElementById(t);
	if (!n) return {};
	try {
		var r;
		let e = JSON.parse((r = n.textContent) == null ? "" : r);
		if (e && typeof e == "object" && !Array.isArray(e)) return e;
		console.warn(`[islandkit] #${t} must contain a JSON object; ignoring`);
	} catch (e) {
		console.warn(`[islandkit] #${t} is not valid JSON; ignoring`, e);
	}
	return {};
}
function Yo(e, t = {}) {
	var n, r, i, a, o, s;
	let c = Jo(e, Ua), l = (n = e.documentElement.getAttribute("lang")) == null ? void 0 : n, u = (r = (i = e.defaultView) == null || (i = i.location) == null ? void 0 : i.origin) == null ? void 0 : r;
	return qo({
		...c,
		...t,
		locale: (a = (o = (s = t.locale) == null ? c.locale : s) == null ? l : o) == null ? "en" : a,
		routing: {
			origin: u == null ? "" : u,
			...c.routing,
			...t.routing
		}
	});
}
function Xo(e) {
	let t = Jo(e, Wa), n = {};
	for (let [e, r] of Object.entries(t)) typeof r == "string" && (n[e] = r);
	return Object.freeze(n);
}
function Zo(e = window) {
	let t = e[Ya];
	if (typeof t != "object" || !t) return {};
	let n = t, r = {};
	typeof n.onError == "function" && (r.onError = n.onError);
	let i = n.analytics;
	return typeof i == "object" && i && typeof i.track == "function" && (r.analytics = i), Object.keys(r).length === 0 && console.warn(`[islandkit] window.${Ya} is declared but carries neither an onError function nor an analytics object with a track() method; ignoring it.`), r;
}
//#endregion
//#region src/app/mount/auto-mount.ts
var Qo = "200px";
function $o(e, t) {
	var n;
	let r = e.getAttribute(qa);
	return r === null ? ((n = ko(t)) == null ? void 0 : n.lazy) === !0 : r !== "false";
}
async function es(e = {}) {
	var t, n, r;
	let i = (t = e.root) == null ? document : t, a = i instanceof Document ? i : (n = i.ownerDocument) == null ? document : n, o = Yo(a, (r = e.context) == null ? {} : r), s = Xo(a), c = /* @__PURE__ */ new Set(), l = (e) => {
		let t = Ko(e);
		return t ? Go(t, e, {
			host: o,
			messages: s
		}) : Promise.resolve(null);
	}, u = typeof IntersectionObserver < "u", d = [], f = [];
	for (let e of Array.from(i.querySelectorAll(Ha))) {
		let t = Ko(e);
		t && (u && $o(e, t) ? f.push(e) : d.push(e));
	}
	let p = (await Promise.all(d.map(l))).filter((e) => e !== null), m = null;
	if (f.length > 0) {
		var h;
		m = new IntersectionObserver((e, t) => {
			for (let n of e) n.isIntersecting && (t.unobserve(n.target), l(n.target).then((e) => {
				e && c.forEach((t) => t(e));
			}));
		}, { rootMargin: (h = e.lazyRootMargin) == null ? Qo : h });
		for (let e of f) m.observe(e);
	}
	return {
		mounted: p,
		deferred: f,
		onMount(e) {
			return c.add(e), () => c.delete(e);
		},
		stop() {
			m == null || m.disconnect(), m = null;
		}
	};
}
//#endregion
//#region src/shared/lib/text/read.ts
function X(e) {
	return typeof e == "object" && e && !Array.isArray(e) ? e : null;
}
function Z(e) {
	return Array.isArray(e) ? e : [];
}
function Q(e, ...t) {
	for (let n of t) {
		let t = e[n];
		if (t != null && t !== "") return t;
	}
}
var ts = /<(script|style)\b[^>]*>[\s\S]*?<\/\1\s*>/gi, ns = /<\/?[a-z][^>]*>/gi, rs = /&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, is = /[^\S\u00a0]+/g, as = {
	amp: "&",
	lt: "<",
	gt: ">",
	quot: "\"",
	apos: "'",
	nbsp: "\xA0"
}, os = 1114111;
function ss(e, t) {
	var n;
	let r = t.toLowerCase();
	if (r.startsWith("#")) {
		let t = r.startsWith("#x"), n = Number.parseInt(t ? r.slice(2) : r.slice(1), t ? 16 : 10);
		return !Number.isInteger(n) || n <= 0 || n > os ? e : String.fromCodePoint(n);
	}
	return (n = as[r]) == null ? e : n;
}
function $(e) {
	return typeof e == "number" && Number.isFinite(e) ? String(e) : typeof e == "string" ? e.replace(ts, " ").replace(ns, " ").replace(rs, ss).replace(is, " ").trim() : "";
}
function cs(e) {
	return typeof e == "number" && Number.isInteger(e) ? String(e) : typeof e == "string" ? e.trim() : "";
}
function ls(e) {
	return $(e).toLowerCase();
}
//#endregion
//#region src/entities/compatibility/model/normalize.ts
function us(e) {
	let t = [];
	for (let n of Z(e)) {
		let e = X(n);
		if (!e) continue;
		let r = cs(Q(e, "id", "slug", "value"));
		r !== "" && t.push({
			id: r,
			name: $(Q(e, "name", "title", "label")) || r
		});
	}
	return t;
}
function ds(e) {
	let t = [];
	for (let n of Z(e)) {
		let e = X(n), r = $(e ? Q(e, "name", "title") : n);
		if (r === "") continue;
		let i = e ? $(Q(e, "legalName", "legal_name")) : "";
		t.push(i === "" || i === r ? { name: r } : {
			name: r,
			legalName: i
		});
	}
	return t;
}
function fs(e, t) {
	let n = [], r = /* @__PURE__ */ new Set();
	for (let t of Z(e)) {
		let e = X(t), i = cs(e ? Q(e, "id", "slug") : t);
		if (i === "" || r.has(i)) continue;
		if (r.add(i), !e) {
			n.push({
				id: i,
				name: i
			});
			continue;
		}
		let a = $(Q(e, "name", "title", "abbreviation")) || i, o = $(Q(e, "note", "tooltip", "editorial"));
		n.push(o === "" ? {
			id: i,
			name: a
		} : {
			id: i,
			name: a,
			note: o
		});
	}
	for (let e of Z(t)) {
		let t = cs(e);
		t === "" || r.has(t) || (r.add(t), n.push({
			id: t,
			name: t
		}));
	}
	return n;
}
function ps(e) {
	let t = [], n = [], r = Z(Q(e, "categoryIds", "category_ids")), i = Z(Q(e, "categoryNames", "categories"));
	if (r.length > 0) return r.forEach((e, r) => {
		let a = cs(e);
		a === "" || t.includes(a) || (t.push(a), n.push($(i[r])));
	}), {
		ids: t,
		names: n
	};
	for (let e of i) {
		let r = X(e), i = $(r ? Q(r, "name", "title") : e), a = (r ? cs(Q(r, "id", "slug")) : "") || ls(i);
		a === "" || t.includes(a) || (t.push(a), n.push(i));
	}
	return {
		ids: t,
		names: n
	};
}
function ms(e, t) {
	let n = X(e);
	if (!n) return null;
	let r = ds(Q(n, "vendors", "partners")), i = $(Q(n, "solution", "solutionTitle", "solution_title", "title"));
	if (r.length === 0 && i === "") return null;
	let { ids: a, names: o } = ps(n), s = fs(Q(n, "tags", "productTags", "product_tags"), Q(n, "tagIds", "tag_slugs")), c = [...r.flatMap((e) => {
		var t;
		return [e.name, (t = e.legalName) == null ? "" : t];
	}), i].join(" ").toLowerCase();
	return {
		key: `row-${t}`,
		categoryIds: a,
		categoryNames: o,
		vendors: r,
		solution: i,
		version: $(Q(n, "version")),
		tags: s,
		tagIds: s.map((e) => e.id),
		haystack: c
	};
}
function hs(e, t, n) {
	let r = /* @__PURE__ */ new Map(), i = X(e);
	if (!i) return r;
	for (let [e, s] of Object.entries(i)) {
		var a, o;
		let i = (a = t.get(e)) == null ? t.get(ls(e)) : a;
		if (i === void 0) continue;
		let c = (o = r.get(i)) == null ? [] : o;
		for (let e of Z(s)) {
			let t = X(e), r = $(t ? Q(t, "name", "title") : e), i = (t ? cs(Q(t, "id", "slug")) : "") || ls(r);
			i !== "" && (n.has(i) || n.set(i, r || i), c.includes(i) || c.push(i));
		}
		r.set(i, c);
	}
	return r;
}
function gs(e) {
	try {
		return new Intl.Collator(e);
	} catch {
		return new Intl.Collator();
	}
}
function _s(e, t, n, r) {
	let i = /* @__PURE__ */ new Map();
	n.forEach((e, t) => {
		i.has(e.id) || i.set(e.id, t);
	});
	let a = [];
	for (let [n, r] of t) {
		var o;
		r !== 0 && a.push({
			id: n,
			name: (o = e.get(n)) == null ? n : o,
			count: r
		});
	}
	return a.sort((e, t) => {
		let n = i.get(e.id), a = i.get(t.id);
		return n !== void 0 && a !== void 0 ? n - a : n === void 0 ? a === void 0 ? r.compare(e.name, t.name) : 1 : -1;
	}), a;
}
function vs(e, t) {
	let n = /* @__PURE__ */ new Map();
	for (let i of e) {
		var r;
		for (let e of t(i)) n.set(e, ((r = n.get(e)) == null ? 0 : r) + 1);
	}
	return n;
}
var ys = {
	categories: [],
	types: [],
	products: [],
	rows: [],
	dropped: 0
};
function bs(e, t = "en") {
	let n = X(e);
	if (!n) return ys;
	let r = [], i = 0;
	Z(Q(n, "rows", "items")).forEach((e, t) => {
		let n = ms(e, t);
		n === null ? i += 1 : r.push(n);
	});
	let a = us(Q(n, "categories")), o = /* @__PURE__ */ new Map();
	for (let e of a) o.set(e.id, e.name);
	for (let e of r) e.categoryIds.forEach((t, n) => {
		var r;
		let i = (r = e.categoryNames[n]) == null ? "" : r;
		i !== "" && !o.has(t) && o.set(t, i);
	});
	let s = /* @__PURE__ */ new Map();
	for (let e of o.keys()) s.set(e, e);
	for (let [e, t] of o) {
		let n = ls(t);
		n !== "" && !s.has(n) && s.set(n, e);
	}
	let c = us(Q(n, "types")), l = /* @__PURE__ */ new Map();
	for (let e of c) l.set(e.id, e.name);
	let u = hs(Q(n, "typesByCategory", "category_types_mapping"), s, l), d = r.map((e) => {
		let t = [];
		for (let r of e.categoryIds) {
			var n;
			for (let e of (n = u.get(r)) == null ? [] : n) t.includes(e) || t.push(e);
		}
		return {
			...e,
			typeIds: t
		};
	}), f = us(Q(n, "products")), p = /* @__PURE__ */ new Map();
	for (let e of f) p.set(e.id, e.name);
	for (let e of d) for (let t of e.tags) p.has(t.id) || p.set(t.id, t.name);
	let m = gs(t);
	return {
		categories: _s(o, vs(d, (e) => e.categoryIds), a, m),
		types: _s(l, vs(d, (e) => e.typeIds), c, m),
		products: _s(p, vs(d, (e) => e.tagIds), f, m),
		rows: d,
		dropped: i
	};
}
//#endregion
//#region src/features/filter-compatibility/model/selection.ts
var xs = [
	"categories",
	"types",
	"products"
], Ss = {
	search: "",
	categories: [],
	types: [],
	products: []
};
function Cs(e, t) {
	return xs.every((n) => {
		let r = e[n], i = t[n];
		return r.length === i.length && r.every((e, t) => e === i[t]);
	});
}
function ws(e, t) {
	return e.search === t.search && Cs(e, t);
}
function Ts(e, t, n) {
	return {
		...e,
		[t]: n
	};
}
//#endregion
//#region src/features/filter-compatibility/model/filter.ts
var Es = {
	categories: (e) => e.categoryIds,
	types: (e) => e.typeIds,
	products: (e) => e.tagIds
};
function Ds(e, t) {
	return t === "" || e.haystack.includes(t);
}
function Os(e) {
	return e.trim().toLowerCase();
}
function ks(e, t, n) {
	let r = n[t];
	return r.length === 0 || Es[t](e).some((e) => r.includes(e));
}
function As(e, t, n, r) {
	return Ds(e, n) ? xs.every((n) => n === r || ks(e, n, t)) : !1;
}
function js(e, t) {
	let n = Os(t.search);
	return e.filter((e) => As(e, t, n));
}
function Ms(e, t) {
	let n = Os(t.search), r = {
		categories: /* @__PURE__ */ new Set(),
		types: /* @__PURE__ */ new Set(),
		products: /* @__PURE__ */ new Set()
	};
	for (let i of xs) for (let a of e) if (As(a, t, n, i)) for (let e of Es[i](a)) r[i].add(e);
	return r;
}
//#endregion
//#region src/features/filter-compatibility/model/url-sync.ts
var Ns = "search", Ps = {
	categories: "category",
	types: "type",
	products: "tag"
}, Fs = { products: ["tags"] };
function Is(e) {
	let t = [];
	for (let n of e) for (let e of n.split(",")) {
		let n = e.trim();
		n !== "" && !t.includes(n) && t.push(n);
	}
	return t;
}
function Ls(e) {
	return {
		categories: new Set(e.categories.map((e) => e.id)),
		types: new Set(e.types.map((e) => e.id)),
		products: new Set(e.products.map((e) => e.id))
	};
}
function Rs(e, t) {
	var n;
	let r = Ls(t), i = (t) => {
		var n;
		return Is([...e.getAll(Ps[t]), ...((n = Fs[t]) == null ? [] : n).flatMap((t) => e.getAll(t))]).filter((e) => r[t].has(e));
	};
	return {
		search: ((n = e.get("search")) == null ? "" : n).trim(),
		categories: i("categories"),
		types: i("types"),
		products: i("products")
	};
}
function zs(e) {
	let t = e.search.trim(), n = { [Ns]: t === "" ? void 0 : t };
	for (let t of xs) {
		var r;
		let i = e[t];
		n[Ps[t]] = i.length === 0 ? void 0 : i.join(",");
		for (let e of (r = Fs[t]) == null ? [] : r) n[e] = void 0;
	}
	return n;
}
function Bs(e, t, n, r = {}) {
	var i;
	let a = (i = r.debounceMs) == null ? 300 : i;
	n.value = Rs(e, t);
	let o, s = !1, c = (t) => {
		s = !0;
		try {
			e.set(zs(t));
		} finally {
			s = !1;
		}
	}, l = () => {
		o !== void 0 && clearTimeout(o), o = void 0;
	};
	Mn(n, (e, t) => {
		if (l(), !Cs(e, t)) {
			c(e);
			return;
		}
		o = setTimeout(() => {
			o = void 0, c(n.value);
		}, a);
	});
	let u = e.subscribe(() => {
		if (s) return;
		let r = Rs(e, t);
		ws(r, n.value) || (n.value = r);
	});
	Ae(() => {
		l(), u();
	});
}
//#endregion
//#region src/shared/kit/button/IkButton.vue?vue&type=script&setup=true&lang.ts
var Vs = [
	"href",
	"target",
	"rel"
], Hs = ["type", "disabled"], Us = /* @__PURE__ */ R({
	__name: "IkButton",
	props: {
		variant: { default: "primary" },
		type: { default: "button" },
		disabled: {
			type: Boolean,
			default: !1
		},
		href: { default: "" },
		target: { default: "" },
		rel: { default: "" }
	},
	emits: ["click"],
	setup(e) {
		let t = e;
		return (e, n) => t.href === "" ? (U(), W("button", {
			key: 1,
			type: t.type,
			disabled: t.disabled,
			class: w(["ik-button", `ik-button--${t.variant}`]),
			onClick: n[1] || (n[1] = (t) => e.$emit("click", t))
		}, [nr(e.$slots, "default")], 10, Hs)) : (U(), W("a", {
			key: 0,
			href: t.href,
			target: t.target === "" ? void 0 : t.target,
			rel: t.rel === "" ? void 0 : t.rel,
			class: w(["ik-button", `ik-button--${t.variant}`]),
			onClick: n[0] || (n[0] = (t) => e.$emit("click", t))
		}, [nr(e.$slots, "default")], 10, Vs));
	}
}), Ws = ["id"], Gs = [
	"id",
	"aria-expanded",
	"aria-controls",
	"aria-labelledby"
], Ks = ["id", "aria-labelledby"], qs = { class: "ik-filter-select__option ik-filter-select__option--all" }, Js = ["checked"], Ys = { class: "ik-filter-select__text" }, Xs = { class: "ik-filter-select__list" }, Zs = [
	"value",
	"checked",
	"disabled",
	"onChange"
], Qs = { class: "ik-filter-select__text" }, $s = { class: "ik-filter-select__count" }, ec = /* @__PURE__ */ R({
	__name: "IkFilterSelect",
	props: {
		label: {},
		placeholder: {},
		options: {},
		modelValue: {},
		allLabel: { default: "" },
		enabled: { default: null }
	},
	emits: ["update:modelValue"],
	setup(e, { emit: t }) {
		let n = e, r = t, { t: i } = go(), a = Bn(), o = Bn(), s = Bn(), c = /* @__PURE__ */ N(null), l = /* @__PURE__ */ N(null), u = /* @__PURE__ */ N(null), d = /* @__PURE__ */ N(!1), f = Ri(() => n.modelValue.length), p = Ri(() => n.options.length > 0 && f.value === n.options.length), m = Ri(() => f.value === 0 ? n.placeholder : p.value && n.allLabel !== "" ? n.allLabel : f.value <= 3 ? n.options.filter((e) => n.modelValue.includes(e.id)).map((e) => e.name).join(", ") : i("kit.selected", { count: f.value }));
		function h(e) {
			return n.modelValue.includes(e);
		}
		function g(e) {
			return n.enabled === null || n.enabled.has(e) || h(e);
		}
		function _(e) {
			r("update:modelValue", h(e) ? n.modelValue.filter((t) => t !== e) : [...n.modelValue, e]);
		}
		function v(e) {
			let t = e.target.checked;
			r("update:modelValue", t ? n.options.map((e) => e.id) : []);
		}
		async function y() {
			var e;
			d.value = !d.value, d.value && (await gn(), (e = u.value) == null || e.focus());
		}
		function b(e) {
			var t;
			d.value && (d.value = !1, e && ((t = l.value) == null || t.focus()));
		}
		function x(e) {
			var t;
			let n = e.relatedTarget;
			n instanceof Node && ((t = c.value) == null ? void 0 : t.contains(n)) === !0 || b(!1);
		}
		return (e, t) => (U(), W("div", {
			ref_key: "root",
			ref: c,
			class: "ik-filter-select",
			onFocusout: x,
			onKeydown: t[0] || (t[0] = Ma(Aa((e) => b(!0), ["stop"]), ["esc"]))
		}, [
			K("span", {
				id: P(a),
				class: "ik-filter-select__label"
			}, T(n.label), 9, Ws),
			K("button", {
				id: P(o),
				ref_key: "trigger",
				ref: l,
				type: "button",
				class: w(["ik-filter-select__trigger", { "ik-filter-select__trigger--open": d.value }]),
				"aria-expanded": d.value,
				"aria-controls": P(s),
				"aria-labelledby": `${P(a)} ${P(o)}`,
				onClick: y
			}, [K("span", { class: w(["ik-filter-select__value", { "ik-filter-select__value--placeholder": f.value === 0 }]) }, T(m.value), 3), t[1] || (t[1] = K("span", {
				class: "ik-filter-select__arrow",
				"aria-hidden": "true"
			}, null, -1))], 10, Gs),
			Dn(K("div", {
				id: P(s),
				role: "group",
				"aria-labelledby": P(a),
				class: "ik-filter-select__panel"
			}, [K("label", qs, [K("input", {
				ref_key: "firstOption",
				ref: u,
				type: "checkbox",
				class: "ik-filter-select__checkbox",
				checked: p.value,
				onChange: v
			}, null, 40, Js), K("span", Ys, T(n.allLabel || P(i)("kit.selectAll")), 1)]), K("ul", Xs, [(U(!0), W(V, null, z(n.options, (e) => (U(), W("li", { key: e.id }, [K("label", { class: w(["ik-filter-select__option", { "ik-filter-select__option--disabled": !g(e.id) }]) }, [
				K("input", {
					type: "checkbox",
					class: "ik-filter-select__checkbox",
					value: e.id,
					checked: h(e.id),
					disabled: !g(e.id),
					onChange: (t) => _(e.id)
				}, null, 40, Zs),
				K("span", Qs, T(e.name), 1),
				K("span", $s, T(e.count), 1)
			], 2)]))), 128))])], 8, Ks), [[$i, d.value]])
		], 544));
	}
}), tc = ["src", "alt"], nc = {
	key: 1,
	class: "ik-icon",
	"aria-hidden": !0,
	focusable: "false"
}, rc = ["href", "xlink:href"], ic = /* @__PURE__ */ R({
	__name: "IkIcon",
	props: {
		icon: {},
		decorative: {
			type: Boolean,
			default: !1
		}
	},
	setup(e) {
		let t = e;
		return (e, n) => t.icon.kind === "image" ? (U(), W("img", {
			key: 0,
			class: "ik-icon",
			src: t.icon.src,
			alt: t.decorative ? "" : t.icon.alt
		}, null, 8, tc)) : (U(), W("svg", nc, [K("use", {
			href: `${t.icon.sprite}#${t.icon.symbol}`,
			"xlink:href": `${t.icon.sprite}#${t.icon.symbol}`
		}, null, 8, rc)]));
	}
}), ac = [
	"id",
	"aria-expanded",
	"aria-controls"
], oc = { class: "ik-menu-panel__label" }, sc = ["id"], cc = ["aria-labelledby"], lc = { class: "ik-menu-panel__title" }, uc = {
	key: 1,
	class: "ik-menu-panel__description"
}, dc = /* @__PURE__ */ R({
	__name: "IkMenuPanel",
	props: {
		label: {},
		items: {},
		layout: { default: "list" },
		hoverable: {
			type: Boolean,
			default: !0
		},
		current: {
			type: Boolean,
			default: !1
		},
		columns: { default: void 0 }
	},
	emits: ["select"],
	setup(e, { emit: t }) {
		let n = e, r = t, i = Bn(), a = Bn(), o = /* @__PURE__ */ N(null), s = /* @__PURE__ */ N(null), c = /* @__PURE__ */ N(!1);
		function l() {
			c.value = !c.value;
		}
		function u(e) {
			var t;
			c.value && (c.value = !1, e && ((t = s.value) == null || t.focus()));
		}
		function d(e) {
			var t;
			let n = e.relatedTarget;
			n instanceof Node && ((t = o.value) == null ? void 0 : t.contains(n)) === !0 || u(!1);
		}
		function f(e) {
			u(!1), r("select", e);
		}
		return (e, t) => (U(), W("div", {
			ref_key: "root",
			ref: o,
			class: w(["ik-menu-panel", {
				"ik-menu-panel--hoverable": n.hoverable,
				"ik-menu-panel--open": c.value,
				"ik-menu-panel--current": n.current
			}]),
			onFocusout: d,
			onKeydown: t[0] || (t[0] = Ma(Aa((e) => u(!0), ["stop"]), ["esc"])),
			onPointerleave: t[1] || (t[1] = (e) => u(!1))
		}, [K("button", {
			id: P(i),
			ref_key: "trigger",
			ref: s,
			type: "button",
			class: "ik-menu-panel__trigger",
			"aria-expanded": c.value,
			"aria-controls": P(a),
			onClick: l
		}, [K("span", oc, T(n.label), 1), t[2] || (t[2] = K("span", {
			class: "ik-menu-panel__arrow",
			"aria-hidden": "true"
		}, null, -1))], 8, ac), K("div", {
			id: P(a),
			class: "ik-menu-panel__wrapper"
		}, [K("ul", {
			class: w(["ik-menu-panel__list", [`ik-menu-panel__list--${n.layout}`, n.layout === "grid" && n.columns ? `ik-menu-panel__list--cols-${n.columns}` : ""]]),
			"aria-labelledby": P(i)
		}, [(U(!0), W(V, null, z(n.items, (e) => (U(), W("li", {
			key: e.key,
			class: "ik-menu-panel__item"
		}, [(U(), G($n(e.url === "" ? "span" : "a"), {
			class: w(["ik-menu-panel__link", { "ik-menu-panel__link--plain": n.layout === "grid" && !e.icon }]),
			href: e.url === "" ? void 0 : e.url,
			target: e.target,
			rel: e.rel,
			onClick: (t) => f(e)
		}, {
			default: L(() => [
				e.icon ? (U(), G(P(ic), {
					key: 0,
					icon: e.icon,
					decorative: ""
				}, null, 8, ["icon"])) : Y("", !0),
				K("span", lc, T(e.title), 1),
				e.description ? (U(), W("span", uc, T(e.description), 1)) : Y("", !0)
			]),
			_: 2
		}, 1032, [
			"class",
			"href",
			"target",
			"rel",
			"onClick"
		]))]))), 128))], 10, cc)], 8, sc)], 34));
	}
}), fc = [
	"aria-expanded",
	"aria-controls",
	"onClick"
], pc = { class: "ik-nav-tree__title" }, mc = {
	key: 1,
	class: "ik-nav-tree__description"
}, hc = [
	"aria-expanded",
	"aria-controls",
	"aria-label",
	"onClick"
], gc = ["id", "hidden"], _c = /* @__PURE__ */ R({
	__name: "IkNavTree",
	props: {
		items: {},
		currentKeys: { default: () => /* @__PURE__ */ new Set() },
		sections: {
			type: Boolean,
			default: !1
		},
		rowToggle: {
			type: Boolean,
			default: !1
		},
		openTrail: {
			type: Boolean,
			default: !0
		},
		level: { default: 0 }
	},
	emits: ["select"],
	setup(e, { emit: t }) {
		let n = e, r = t, { t: i } = go(), a = Bn(), o = /* @__PURE__ */ N(/* @__PURE__ */ new Map()), s = Ri(() => {
			let e = /* @__PURE__ */ new Set();
			for (let t of n.items) {
				let r = o.value.get(t.key), i = n.openTrail && n.currentKeys.has(t.key);
				(n.sections || (r == null ? i : r)) && e.add(t.key);
			}
			return e;
		});
		function c(e) {
			var t, n;
			return ((t = (n = e.items) == null ? void 0 : n.length) == null ? 0 : t) > 0;
		}
		function l(e) {
			return n.rowToggle && !n.sections && e.url === "" && c(e);
		}
		let u = Ri(() => n.items.some((e) => e.description !== void 0 || e.icon !== void 0));
		function d(e) {
			let t = new Map(o.value);
			t.set(e.key, !s.value.has(e.key)), o.value = t;
		}
		function f(e) {
			r("select", e);
		}
		return (e, t) => {
			let r = Zn("IkNavTree", !0);
			return U(), W("ul", { class: w(["ik-nav-tree", [`ik-nav-tree--level-${n.level}`, { "ik-nav-tree--cards": u.value }]]) }, [(U(!0), W(V, null, z(n.items, (e) => {
				var o;
				return U(), W("li", {
					key: e.key,
					class: w(["ik-nav-tree__item", {
						"ik-nav-tree__item--current": n.currentKeys.has(e.key),
						"ik-nav-tree__item--open": s.value.has(e.key),
						"ik-nav-tree__item--parent": c(e)
					}])
				}, [K("div", { class: w(["ik-nav-tree__row", { "ik-nav-tree__row--section": n.sections && c(e) }]) }, [l(e) ? (U(), W("button", {
					key: 0,
					type: "button",
					class: "ik-nav-tree__link ik-nav-tree__link--toggle",
					"aria-expanded": s.value.has(e.key),
					"aria-controls": `${P(a)}-${e.key}`,
					onClick: (t) => d(e)
				}, [
					e.icon ? (U(), G(P(ic), {
						key: 0,
						icon: e.icon,
						decorative: ""
					}, null, 8, ["icon"])) : Y("", !0),
					K("span", pc, T(e.title), 1),
					t[0] || (t[0] = K("span", {
						class: "ik-nav-tree__arrow",
						"aria-hidden": "true"
					}, null, -1))
				], 8, fc)) : (U(), G($n(e.url === "" ? "span" : "a"), {
					key: 1,
					class: "ik-nav-tree__link",
					href: e.url === "" ? void 0 : e.url,
					target: e.target,
					rel: e.rel,
					"aria-current": n.currentKeys.has(e.key) && e.url !== "" ? "page" : void 0,
					onClick: (t) => f(e)
				}, {
					default: L(() => [
						e.icon ? (U(), G(P(ic), {
							key: 0,
							icon: e.icon,
							decorative: ""
						}, null, 8, ["icon"])) : Y("", !0),
						K("span", { class: w(["ik-nav-tree__title", { "ik-nav-tree__title--section": n.sections && c(e) }]) }, T(e.title), 3),
						e.description ? (U(), W("span", mc, T(e.description), 1)) : Y("", !0)
					]),
					_: 2
				}, 1032, [
					"href",
					"target",
					"rel",
					"aria-current",
					"onClick"
				])), c(e) && !n.sections && !l(e) ? (U(), W("button", {
					key: 2,
					type: "button",
					class: "ik-nav-tree__toggle",
					"aria-expanded": s.value.has(e.key),
					"aria-controls": `${P(a)}-${e.key}`,
					"aria-label": P(i)("kit.submenu", { name: e.title }),
					onClick: (t) => d(e)
				}, [...t[1] || (t[1] = [K("span", {
					class: "ik-nav-tree__arrow",
					"aria-hidden": "true"
				}, null, -1)])], 8, hc)) : Y("", !0)], 2), c(e) ? (U(), W("div", {
					key: 0,
					id: `${P(a)}-${e.key}`,
					class: "ik-nav-tree__children",
					hidden: !s.value.has(e.key)
				}, [q(r, {
					items: (o = e.items) == null ? [] : o,
					"current-keys": n.currentKeys,
					sections: n.sections,
					"row-toggle": n.rowToggle,
					"open-trail": n.openTrail,
					level: n.level + 1,
					onSelect: f
				}, null, 8, [
					"items",
					"current-keys",
					"sections",
					"row-toggle",
					"open-trail",
					"level"
				])], 8, gc)) : Y("", !0)], 2);
			}), 128))], 2);
		};
	}
}), vc = { class: "ik-search-field" }, yc = ["for"], bc = { class: "ik-search-field__box" }, xc = [
	"id",
	"value",
	"placeholder"
], Sc = ["aria-label"], Cc = /* @__PURE__ */ R({
	__name: "IkSearchField",
	props: {
		modelValue: {},
		label: {},
		placeholder: {}
	},
	emits: ["update:modelValue"],
	setup(e, { emit: t }) {
		let n = e, r = t, { t: i } = go(), a = Bn();
		function o(e) {
			r("update:modelValue", e.target.value);
		}
		return (e, t) => (U(), W("div", vc, [K("label", {
			for: P(a),
			class: "ik-search-field__label"
		}, T(n.label), 9, yc), K("div", bc, [K("input", {
			id: P(a),
			type: "search",
			class: "ik-search-field__input",
			value: n.modelValue,
			placeholder: n.placeholder,
			onInput: o
		}, null, 40, xc), n.modelValue === "" ? Y("", !0) : (U(), W("button", {
			key: 0,
			type: "button",
			class: "ik-search-field__clear",
			"aria-label": P(i)("kit.clear"),
			onClick: t[0] || (t[0] = (e) => r("update:modelValue", ""))
		}, [...t[1] || (t[1] = [K("span", { "aria-hidden": "true" }, "×", -1)])], 8, Sc))])]));
	}
}), wc = /* @__PURE__ */ R({
	__name: "IkTag",
	props: { variant: { default: "outline" } },
	setup(e) {
		let t = e;
		return (e, n) => (U(), W("span", { class: w(["ik-tag", `ik-tag--${t.variant}`]) }, [nr(e.$slots, "default")], 2));
	}
}), Tc = ["aria-describedby"], Ec = ["id"], Dc = /* @__PURE__ */ R({
	__name: "IkTooltip",
	props: {
		note: {},
		align: { default: "center" }
	},
	setup(e) {
		let t = e, n = Bn(), r = /* @__PURE__ */ N(!1);
		return (e, i) => (U(), W("span", {
			class: w(["ik-tooltip", { "ik-tooltip--open": r.value }]),
			onMouseenter: i[3] || (i[3] = (e) => r.value = !0),
			onMouseleave: i[4] || (i[4] = (e) => r.value = !1),
			onKeydown: i[5] || (i[5] = Ma(Aa((e) => r.value = !1, ["stop"]), ["esc"]))
		}, [K("button", {
			type: "button",
			class: "ik-tooltip__trigger",
			"aria-describedby": P(n),
			onFocus: i[0] || (i[0] = (e) => r.value = !0),
			onBlur: i[1] || (i[1] = (e) => r.value = !1),
			onClick: i[2] || (i[2] = (e) => r.value = !r.value)
		}, [nr(e.$slots, "default")], 40, Tc), K("span", {
			id: P(n),
			role: "tooltip",
			class: w(["ik-tooltip__bubble", [`ik-tooltip__bubble--${t.align}`, { "ik-tooltip__bubble--open": r.value }]])
		}, T(t.note), 11, Ec)], 34));
	}
}), Oc = { class: "ik-compat-filters" }, kc = /* @__PURE__ */ R({
	__name: "CompatibilityFilters",
	props: {
		data: {},
		selection: {},
		availability: {}
	},
	emits: ["update:selection"],
	setup(e, { emit: t }) {
		let n = e, r = t, { t: i } = go();
		function a(e, t) {
			r("update:selection", Ts(n.selection, e, t));
		}
		function o(e) {
			r("update:selection", {
				...n.selection,
				search: e
			});
		}
		return (e, t) => (U(), W("div", Oc, [
			q(P(Cc), {
				class: "ik-compat-filters__field",
				"model-value": n.selection.search,
				label: P(i)("compat.searchLabel"),
				placeholder: P(i)("compat.searchPlaceholder"),
				"onUpdate:modelValue": o
			}, null, 8, [
				"model-value",
				"label",
				"placeholder"
			]),
			n.data.categories.length > 0 ? (U(), G(P(ec), {
				key: 0,
				class: "ik-compat-filters__field",
				label: P(i)("compat.categoryLabel"),
				placeholder: P(i)("compat.categoryPlaceholder"),
				"all-label": P(i)("compat.categoryAll"),
				options: n.data.categories,
				"model-value": n.selection.categories,
				enabled: n.availability.categories,
				"onUpdate:modelValue": t[0] || (t[0] = (e) => a("categories", e))
			}, null, 8, [
				"label",
				"placeholder",
				"all-label",
				"options",
				"model-value",
				"enabled"
			])) : Y("", !0),
			n.data.types.length > 0 ? (U(), G(P(ec), {
				key: 1,
				class: "ik-compat-filters__field",
				label: P(i)("compat.typeLabel"),
				placeholder: P(i)("compat.typePlaceholder"),
				"all-label": P(i)("compat.typeAll"),
				options: n.data.types,
				"model-value": n.selection.types,
				enabled: n.availability.types,
				"onUpdate:modelValue": t[1] || (t[1] = (e) => a("types", e))
			}, null, 8, [
				"label",
				"placeholder",
				"all-label",
				"options",
				"model-value",
				"enabled"
			])) : Y("", !0),
			n.data.products.length > 0 ? (U(), G(P(ec), {
				key: 2,
				class: "ik-compat-filters__field",
				label: P(i)("compat.productLabel"),
				placeholder: P(i)("compat.productPlaceholder"),
				"all-label": P(i)("compat.productAll"),
				options: n.data.products,
				"model-value": n.selection.products,
				enabled: n.availability.products,
				"onUpdate:modelValue": t[2] || (t[2] = (e) => a("products", e))
			}, null, 8, [
				"label",
				"placeholder",
				"all-label",
				"options",
				"model-value",
				"enabled"
			])) : Y("", !0)
		]));
	}
}), Ac = { class: "ik-compat" }, jc = {
	class: "ik-sr-only",
	role: "status"
}, Mc = {
	role: "table",
	class: "ik-compat-table"
}, Nc = {
	role: "rowgroup",
	class: "ik-compat-table__head"
}, Pc = { role: "row" }, Fc = {
	role: "columnheader",
	scope: "col"
}, Ic = {
	role: "columnheader",
	scope: "col"
}, Lc = {
	role: "columnheader",
	scope: "col"
}, Rc = {
	role: "columnheader",
	scope: "col"
}, zc = {
	role: "rowgroup",
	class: "ik-compat-table__body"
}, Bc = {
	role: "cell",
	class: "ik-compat-table__cell"
}, Vc = {
	class: "ik-compat-table__label",
	"aria-hidden": "true"
}, Hc = { class: "ik-compat-table__categories" }, Uc = {
	role: "cell",
	class: "ik-compat-table__cell"
}, Wc = {
	class: "ik-compat-table__label",
	"aria-hidden": "true"
}, Gc = { class: "ik-compat-table__solution" }, Kc = {
	role: "cell",
	class: "ik-compat-table__cell ik-compat-table__cell--version"
}, qc = {
	class: "ik-compat-table__label",
	"aria-hidden": "true"
}, Jc = {
	role: "cell",
	class: "ik-compat-table__cell"
}, Yc = {
	class: "ik-compat-table__label",
	"aria-hidden": "true"
}, Xc = {
	key: 0,
	class: "ik-compat-table__tags"
}, Zc = {
	key: 1,
	"aria-hidden": "true"
}, Qc = {
	key: 0,
	role: "row"
}, $c = {
	role: "cell",
	colspan: "4",
	class: "ik-compat-table__empty"
}, el = To({
	id: "compatibility-matrix",
	kind: "interactive",
	component: /* @__PURE__ */ R({
		__name: "CompatibilityTable",
		props: { data: { default: () => ys } },
		setup(e) {
			let t = e, { t: n } = go(), r = _o(), i = yo(), a = /* @__PURE__ */ N(Ss);
			Bs(r, t.data, a);
			let o = Ri(() => js(t.data.rows, a.value)), s = Ri(() => Ms(t.data.rows, a.value));
			return Mn(a, (e) => {
				i("filtered", {
					matched: o.value.length,
					total: t.data.rows.length,
					selection: {
						search: e.search,
						categories: [...e.categories],
						types: [...e.types],
						products: [...e.products]
					}
				});
			}), (e, r) => (U(), W("div", Ac, [
				q(kc, {
					selection: a.value,
					"onUpdate:selection": r[0] || (r[0] = (e) => a.value = e),
					data: t.data,
					availability: s.value
				}, null, 8, [
					"selection",
					"data",
					"availability"
				]),
				K("p", jc, T(P(n)("compat.resultCount", { count: o.value.length })), 1),
				K("table", Mc, [
					r[1] || (r[1] = K("colgroup", null, [
						K("col", { class: "ik-compat-table__col" }),
						K("col", { class: "ik-compat-table__col" }),
						K("col", { class: "ik-compat-table__col" }),
						K("col", { class: "ik-compat-table__col" })
					], -1)),
					K("thead", Nc, [K("tr", Pc, [
						K("th", Fc, T(P(n)("compat.columnCategory")), 1),
						K("th", Ic, T(P(n)("compat.columnVendor")), 1),
						K("th", Lc, T(P(n)("compat.columnVersion")), 1),
						K("th", Rc, T(P(n)("compat.columnCompatibility")), 1)
					])]),
					K("tbody", zc, [(U(!0), W(V, null, z(o.value, (e) => (U(), W("tr", {
						key: e.key,
						role: "row",
						class: "ik-compat-table__row"
					}, [
						K("td", Bc, [K("span", Vc, T(P(n)("compat.columnCategory")), 1), K("span", Hc, [(U(!0), W(V, null, z(e.categoryNames, (e) => (U(), G(P(wc), {
							key: e,
							variant: "subtle"
						}, {
							default: L(() => [J(T(e), 1)]),
							_: 2
						}, 1024))), 128))])]),
						K("td", Uc, [
							K("span", Wc, T(P(n)("compat.columnVendor")), 1),
							(U(!0), W(V, null, z(e.vendors, (e) => (U(), W("span", {
								key: e.name,
								class: "ik-compat-table__vendor"
							}, [J(T(e.name), 1), e.legalName ? (U(), W(V, { key: 0 }, [J(" (" + T(e.legalName) + ")", 1)], 64)) : Y("", !0)]))), 128)),
							K("span", Gc, T(e.solution), 1)
						]),
						K("td", Kc, [K("span", qc, T(P(n)("compat.columnVersion")), 1), J(" " + T(e.version), 1)]),
						K("td", Jc, [K("span", Yc, T(P(n)("compat.columnCompatibility")), 1), e.tags.length > 0 ? (U(), W("span", Xc, [(U(!0), W(V, null, z(e.tags, (e) => (U(), W(V, { key: e.id }, [e.note ? (U(), G(P(Dc), {
							key: 0,
							note: e.note,
							align: "end"
						}, {
							default: L(() => [q(P(wc), null, {
								default: L(() => [J(T(e.name), 1)]),
								_: 2
							}, 1024)]),
							_: 2
						}, 1032, ["note"])) : (U(), G(P(wc), { key: 1 }, {
							default: L(() => [J(T(e.name), 1)]),
							_: 2
						}, 1024))], 64))), 128))])) : (U(), W("span", Zc, "—"))])
					]))), 128)), o.value.length === 0 ? (U(), W("tr", Qc, [K("td", $c, T(P(n)("compat.empty")), 1)])) : Y("", !0)])
				])
			]));
		}
	}),
	minHeight: 560,
	urlSync: !0,
	parseProps: (e, t) => ({ data: bs(e, t.locale) }),
	messageKeys: [
		"compat.searchLabel",
		"compat.searchPlaceholder",
		"compat.categoryLabel",
		"compat.categoryPlaceholder",
		"compat.categoryAll",
		"compat.typeLabel",
		"compat.typePlaceholder",
		"compat.typeAll",
		"compat.productLabel",
		"compat.productPlaceholder",
		"compat.productAll",
		"compat.columnCategory",
		"compat.columnVendor",
		"compat.columnVersion",
		"compat.columnCompatibility",
		"compat.resultCount",
		"compat.empty"
	]
});
//#endregion
//#region src/shared/lib/icon/read.ts
function tl(e, t = "") {
	if (typeof e == "string") {
		let n = e.trim();
		return n === "" ? void 0 : {
			kind: "image",
			src: n,
			alt: t
		};
	}
	let n = X(e);
	if (n === null) return;
	let r = $(Q(n, "sprite", "spriteUrl", "sprite_url")), i = cs(Q(n, "symbol", "symbolId", "symbol_id", "id"));
	if (r !== "" && i !== "") return {
		kind: "sprite",
		sprite: r,
		symbol: i
	};
	let a = $(Q(n, "src", "url", "image"));
	if (a !== "") return {
		kind: "image",
		src: a,
		alt: $(Q(n, "alt")) || t
	};
}
//#endregion
//#region src/entities/navigation/model/normalize.ts
var nl = 3, rl = ["#", "/#"], il = /^[a-z][a-z0-9+.-]*:\/\/[^/]*/i, al = [
	"items",
	"children",
	"sub_items"
];
function ol(e) {
	var t;
	let n = typeof e == "string" ? e.trim() : "";
	return n === "" ? "" : ((t = n.replace(il, "").split(/[?#]/, 1)[0]) == null ? "" : t).replace(/^\/+/, "").replace(/\/+$/, "");
}
function sl(e) {
	let t = $(e);
	return t === "" || rl.includes(t) ? "" : t;
}
function cl(e, t) {
	if (t.length === 0) return;
	let n = $(e).toLowerCase();
	return n === "grid" || n === "list" ? n : t.some((e) => e.icon !== void 0 || e.description !== void 0) ? "grid" : "list";
}
function ll(e) {
	let t = $(e), n = /^\d$/.test(t) ? Number(t) : 0;
	return n === 1 || n === 2 || n === 3 || n === 4 ? n : void 0;
}
function ul(e) {
	let t = 0;
	for (let n of Z(e)) {
		t += 1;
		let e = X(n);
		e && (t += ul(Q(e, ...al)));
	}
	return t;
}
function dl(e, t, n) {
	let r = [], i = 0;
	return Z(e).forEach((e, a) => {
		let o = ml(e, t + String(a), n);
		i += o.dropped, o.item !== null && r.push(o.item);
	}), {
		items: r,
		dropped: i
	};
}
function fl(e) {
	let t = Q(e, "current", "isCurrent", "is_current");
	if (t === !0 || typeof t == "string" && [
		"true",
		"1",
		"yes"
	].includes(t.trim().toLowerCase()) || t === 1) return !0;
}
function pl(e) {
	let t = Q(e, "currentSlot", "current_slot");
	if (t === !0 || t === 1 || typeof t == "string" && [
		"true",
		"1",
		"yes"
	].includes(t.trim().toLowerCase())) return !0;
}
function ml(e, t, n) {
	let r = X(e);
	if (!r) return {
		item: null,
		dropped: 1
	};
	let i = cs(Q(r, "id", "key")) || t, a = $(Q(r, "title", "name", "label")), o = sl(Q(r, "url", "href")), s = Q(r, ...al), c = [], l = 0;
	if (n < nl) {
		let e = dl(s, t + "-", n + 1);
		c = e.items, l += e.dropped;
	} else l += ul(s);
	if (a === "" || o === "" && c.length === 0) return {
		item: null,
		dropped: l + 1
	};
	let u = $(Q(r, "target")), d = $(Q(r, "rel", "xfn")), f = $(Q(r, "description", "text", "excerpt")), p = tl(Q(r, "icon", "logo", "image"), a), m = tl(Q(r, "lockup", "logo_full"), a), h = cl(Q(r, "panel", "layout"), c), g = h === "grid" ? ll(Q(r, "columns", "cols")) : void 0, _ = fl(r), v = $(Q(r, "section")), y = pl(r);
	return {
		item: {
			key: i,
			title: a,
			url: o,
			...u === "" ? {} : { target: u },
			...d === "" ? {} : { rel: d },
			...f === "" ? {} : { description: f },
			...p === void 0 ? {} : { icon: p },
			...(m == null ? void 0 : m.kind) === "image" ? { lockup: m } : {},
			...h === void 0 ? {} : { panel: h },
			...g === void 0 ? {} : { columns: g },
			..._ ? { current: _ } : {},
			...v === "" ? {} : { section: v },
			...y ? { currentSlot: y } : {},
			items: c
		},
		dropped: l
	};
}
function hl(e) {
	let t = /* @__PURE__ */ new Map(), n = [];
	for (let r of e) {
		let e = X(r);
		if (!e) continue;
		let i = {
			...e,
			items: [...Z(Q(e, "items", "children"))]
		};
		n.push(i);
		let a = cs(Q(e, "id"));
		a !== "" && t.set(a, i);
	}
	let r = [];
	for (let e of n) {
		var i;
		let n = cs((i = e.parent_id) == null ? e.parentId : i), a = n === "" ? void 0 : t.get(n);
		a === void 0 || a === e ? r.push(e) : a.items.push(e);
	}
	return r;
}
function gl(e) {
	return e.some((e) => {
		var t;
		let n = X(e);
		if (!n) return !1;
		let r = cs((t = n.parent_id) == null ? n.parentId : t);
		return r !== "" && r !== "0";
	});
}
var _l = {
	items: [],
	dropped: 0
};
function vl(e, t = {}) {
	var n, r;
	let i = X(e), a = Z(i === null ? e : (n = Q(i, "items", "menu", "nav")) == null ? [] : n);
	if (a.length === 0) return _l;
	let o = dl(gl(a) ? hl(a) : a, (r = t.keyPrefix) == null ? "nav-" : r, 1);
	return {
		items: o.items,
		dropped: o.dropped
	};
}
//#endregion
//#region src/entities/navigation/model/active.ts
var yl = {
	keys: /* @__PURE__ */ new Set(),
	activeKey: null,
	matched: null
};
function bl(e, t) {
	let n = ol(e), r = ol(t);
	return n !== "" && n === r;
}
function xl(e, t) {
	let n = ol(e), r = ol(t);
	return n === "" ? !1 : r === n || r.startsWith(n + "/");
}
function Sl(e, t, n, r, i) {
	let a = i;
	for (let i of e) {
		let e = [...n, i.key];
		t(i) && (a === null || r > a.depth) && (a = {
			depth: r,
			trail: e
		}), a = Sl(i.items, t, e, r + 1, a);
	}
	return a;
}
function Cl(e, t) {
	for (let n of e) {
		if (n.current === !0) {
			let e = ol(n.url);
			e !== "" && t.push(e);
		}
		Cl(n.items, t);
	}
	return t;
}
function wl(e, t, n, r, i) {
	let a = i;
	for (let i of e) {
		let e = [...r, i.key];
		if (!n.some((e) => xl(i.url, e))) for (let n of i.section === void 0 ? [i.url] : [i.url, i.section]) {
			if (!xl(n, t)) continue;
			let r = ol(n).length;
			(a === null || r > a.length) && (a = {
				length: r,
				trail: e
			});
		}
		a = wl(i.items, t, n, e, a);
	}
	return a;
}
function Tl(e, t) {
	var n, r;
	if (e.length === 0) return yl;
	let i = ol(t), a = i === "" ? null : Sl(e, (e) => bl(e.url, i), [], 0, null), o = a !== null || i === "" ? null : wl(e, i, Cl(e, []), [], null), s = (n = a == null ? o : a) == null ? Sl(e, (e) => e.current === !0, [], 0, null) : n;
	return s === null ? yl : {
		keys: new Set(s.trail),
		activeKey: (r = s.trail[s.trail.length - 1]) == null ? null : r,
		matched: a === null ? o === null ? "flag" : "section" : "path"
	};
}
function El(e, t) {
	let n = ol(t);
	if (n === "") return null;
	let r = null, i = -1;
	for (let t of e) {
		if (!xl(t.url, n)) continue;
		let e = ol(t.url).length;
		e > i && (r = t, i = e);
	}
	return r;
}
//#endregion
//#region src/app/islands/menu-panel.ts
var Dl = To({
	id: "menu-panel",
	kind: "content",
	component: dc,
	parseProps: (e) => {
		var t, n;
		let r = vl((t = (n = e.items) == null ? e.menu : n) == null ? e : t), i = (typeof e.layout == "string" ? e.layout.toLowerCase() : "") === "grid" ? "grid" : "list";
		return {
			label: typeof e.label == "string" ? e.label : "",
			items: r.items,
			layout: i,
			hoverable: e.hoverable === !0
		};
	}
}), Ol = {
	l: 1024,
	xl: 1200
};
function kl(e = "l") {
	return `(min-width: ${Ol[e]}px)`;
}
kl("l");
//#endregion
//#region src/widgets/site-header/ui/HeaderProductRow.vue?vue&type=script&setup=true&lang.ts
var Al = ["onKeydown"], jl = ["hidden"], Ml = { class: "ik-header__row2-inner" }, Nl = { class: "ik-header__product" }, Pl = ["src", "alt"], Fl = {
	key: 2,
	class: "ik-header__product-title"
}, Il = ["aria-expanded", "aria-controls"], Ll = ["id"], Rl = [
	"aria-expanded",
	"aria-controls",
	"aria-label",
	"onClick"
], zl = ["id"], Bl = { class: "ik-header__row2-subdesc" }, Vl = {
	key: 2,
	class: "ik-header__row2-slot",
	"data-ik-slot": "row2-current"
}, Hl = /* @__PURE__ */ R({
	__name: "HeaderProductRow",
	props: {
		product: {},
		trail: {},
		breakpoint: { default: "l" }
	},
	setup(e) {
		let t = e, { t: n } = go(), r = Bn(), i = /* @__PURE__ */ N(null), a = /* @__PURE__ */ N(!1), o = /* @__PURE__ */ N(/* @__PURE__ */ new Map()), s = Co(kl(t.breakpoint)), c = Ri(() => {
			var e, n;
			let r = t.product.items.find((e) => e.key === t.trail.activeKey);
			if (r !== void 0) return r.title;
			for (let e of t.product.items) if (t.trail.keys.has(e.key)) return e.title;
			return (e = (n = t.product.items[0]) == null ? void 0 : n.title) == null ? t.product.title : e;
		});
		function l(e) {
			return o.value.get(e.key) === !0;
		}
		function u(e) {
			return t.trail.keys.has(e.key) && o.value.get(e.key) !== !1;
		}
		function d(e) {
			return s.value ? l(e) : l(e) || u(e);
		}
		function f(e) {
			let t = d(e), n = s.value && !t ? /* @__PURE__ */ new Map() : new Map(o.value);
			n.set(e.key, !t), o.value = n;
		}
		Mn(s, (e) => {
			e && p();
		});
		function p() {
			a.value = !1, s.value && o.value.size > 0 && (o.value = /* @__PURE__ */ new Map());
		}
		function m() {
			s.value && p();
		}
		function h(e, t) {
			e.items.length !== 0 && (t.metaKey || t.ctrlKey || t.shiftKey || t.button !== 0 || (t.preventDefault(), f(e)));
		}
		function g(e) {
			var t;
			let n = e.relatedTarget;
			n instanceof Node && ((t = i.value) == null ? void 0 : t.contains(n)) === !0 || p();
		}
		return (e, o) => (U(), W("div", {
			ref_key: "root",
			ref: i,
			class: w(["ik-header__row2", { "ik-header__row2--open": a.value }]),
			onKeydown: Ma(Aa(p, ["stop"]), ["esc"]),
			onFocusout: g,
			onPointerleave: m
		}, [K("div", {
			class: "ik-header__row2-backdrop",
			hidden: !a.value,
			onClick: p
		}, null, 8, jl), K("div", Ml, [
			K("div", Nl, [t.product.lockup === void 0 && t.product.icon ? (U(), G(P(ic), {
				key: 0,
				icon: t.product.icon,
				decorative: ""
			}, null, 8, ["icon"])) : t.product.lockup ? (U(), W("img", {
				key: 1,
				class: "ik-header__product-lockup",
				src: t.product.lockup.src,
				alt: t.product.lockup.alt
			}, null, 8, Pl)) : Y("", !0), t.product.lockup === void 0 ? (U(), W("span", Fl, T(t.product.title), 1)) : Y("", !0)]),
			K("button", {
				type: "button",
				class: "ik-header__row2-trigger",
				"aria-expanded": a.value,
				"aria-controls": P(r),
				onClick: o[0] || (o[0] = (e) => a.value = !a.value)
			}, [K("span", null, T(c.value), 1), o[1] || (o[1] = K("span", {
				class: "ik-header__row2-arrow",
				"aria-hidden": "true"
			}, null, -1))], 8, Il),
			K("ul", {
				id: P(r),
				class: "ik-header__row2-nav"
			}, [(U(!0), W(V, null, z(t.product.items, (e) => (U(), W("li", {
				key: e.key,
				class: w(["ik-header__row2-item", {
					"ik-header__row2-item--current": t.trail.keys.has(e.key),
					"ik-header__row2-item--parent": e.items.length > 0,
					"ik-header__row2-item--open": l(e),
					"ik-header__row2-item--trail": u(e)
				}])
			}, [
				(U(), G($n(e.url === "" ? "span" : "a"), {
					class: "ik-header__row2-link",
					href: e.url === "" ? void 0 : e.url,
					target: e.target,
					rel: e.rel,
					"aria-current": t.trail.activeKey === e.key && e.url !== "" ? t.trail.matched === "path" ? "page" : "location" : void 0,
					onClick: (t) => h(e, t)
				}, {
					default: L(() => [J(T(e.title), 1)]),
					_: 2
				}, 1032, [
					"href",
					"target",
					"rel",
					"aria-current",
					"onClick"
				])),
				e.items.length > 0 ? (U(), W("button", {
					key: 0,
					type: "button",
					class: "ik-header__row2-toggle",
					"aria-expanded": d(e),
					"aria-controls": `${P(r)}-${e.key}`,
					"aria-label": P(n)("kit.submenu", { name: e.title }),
					onClick: (t) => f(e)
				}, [...o[2] || (o[2] = [K("span", {
					class: "ik-header__row2-arrow",
					"aria-hidden": "true"
				}, null, -1)])], 8, Rl)) : Y("", !0),
				e.items.length > 0 ? (U(), W("div", {
					key: 1,
					id: `${P(r)}-${e.key}`,
					class: w(["ik-header__row2-sub", { "ik-header__row2-sub--grid": e.panel === "grid" }])
				}, [K("ul", null, [(U(!0), W(V, null, z(e.items, (n) => (U(), W("li", {
					key: n.key,
					class: w({ "ik-header__row2-subitem--current": t.trail.keys.has(n.key) })
				}, [(U(), G($n(n.url === "" ? "span" : "a"), {
					class: "ik-header__row2-link",
					href: n.url === "" ? void 0 : n.url,
					target: n.target,
					rel: n.rel,
					"aria-current": t.trail.activeKey === n.key && n.url !== "" ? t.trail.matched === "path" ? "page" : "location" : void 0
				}, {
					default: L(() => [e.panel === "grid" && n.description ? (U(), W(V, { key: 0 }, [J(T(n.title) + " ", 1), K("span", Bl, T(n.description), 1)], 64)) : (U(), W(V, { key: 1 }, [J(T(n.title), 1)], 64))]),
					_: 2
				}, 1032, [
					"href",
					"target",
					"rel",
					"aria-current"
				]))], 2))), 128))])], 10, zl)) : Y("", !0),
				t.product.currentSlot && t.trail.keys.has(e.key) ? (U(), W("div", Vl)) : Y("", !0)
			], 2))), 128))], 8, Ll)
		])], 42, Al));
	}
}), Ul = ["aria-expanded", "aria-label"], Wl = ["hidden"], Gl = ["hidden"], Kl = { class: "ik-header-sheet__body" }, ql = { class: "ik-header-sheet__footer" }, Jl = {
	key: 0,
	class: "ik-header-sheet__actions"
}, Yl = ["href", "aria-label"], Xl = { "aria-hidden": "true" }, Zl = { "aria-hidden": "true" }, Ql = {
	key: 2,
	class: "ik-header-sheet__social"
}, $l = [
	"href",
	"target",
	"rel",
	"title"
], eu = { key: 1 }, tu = "a[href], button:not([disabled]), [tabindex=\"0\"]:not([data-ik-sentinel])", nu = /* @__PURE__ */ R({
	__name: "HeaderSheet",
	props: {
		menu: {},
		social: {},
		actions: {},
		language: {},
		trail: {},
		sections: {
			type: Boolean,
			default: !0
		},
		openTrail: {
			type: Boolean,
			default: !0
		},
		breakpoint: { default: "l" }
	},
	emits: ["action"],
	setup(e, { emit: t }) {
		let n = e, r = t, { t: i } = go(), a = /* @__PURE__ */ N(!1), o = /* @__PURE__ */ N(!1), s = /* @__PURE__ */ N(null), c = /* @__PURE__ */ N(null);
		xo(a);
		function l() {
			var e;
			let t = (e = s.value) == null ? void 0 : e.querySelectorAll(tu);
			return t === void 0 ? [] : Array.from(t);
		}
		async function u() {
			var e;
			o.value = !0, a.value = !0, await gn(), (e = l()[0]) == null || e.focus();
		}
		function d(e) {
			var t;
			a.value && (a.value = !1, e && ((t = c.value) == null || t.focus()));
		}
		function f() {
			a.value ? d(!0) : u();
		}
		Mn(Co(kl(n.breakpoint)), (e) => {
			e && d(!1);
		});
		function p(e) {
			let t = l(), n = e === "end" ? t[0] : t[t.length - 1];
			n == null || n.focus();
		}
		let m = Ri(() => a.value ? i("kit.closeMenu") : i("kit.openMenu"));
		return (e, t) => (U(), W("div", { class: w(["ik-header-sheet", { "ik-header-sheet--open": a.value }]) }, [K("button", {
			ref_key: "trigger",
			ref: c,
			type: "button",
			class: "ik-header-sheet__trigger",
			"aria-expanded": a.value,
			"aria-label": m.value,
			onClick: f
		}, [...t[4] || (t[4] = [
			K("span", {
				class: "ik-header-sheet__bar",
				"aria-hidden": "true"
			}, null, -1),
			K("span", {
				class: "ik-header-sheet__bar",
				"aria-hidden": "true"
			}, null, -1),
			K("span", {
				class: "ik-header-sheet__bar",
				"aria-hidden": "true"
			}, null, -1)
		])], 8, Ul), o.value ? (U(), W(V, { key: 0 }, [K("div", {
			class: "ik-header-sheet__overlay",
			hidden: !a.value,
			onClick: t[0] || (t[0] = (e) => d(!1))
		}, null, 8, Wl), K("div", {
			ref_key: "panel",
			ref: s,
			class: "ik-header-sheet__panel",
			hidden: !a.value,
			onKeydown: t[3] || (t[3] = Ma(Aa((e) => d(!0), ["stop"]), ["esc"]))
		}, [
			K("span", {
				"data-ik-sentinel": "",
				tabindex: "0",
				onFocus: t[1] || (t[1] = (e) => p("start"))
			}, null, 32),
			K("div", Kl, [q(P(_c), {
				items: n.menu,
				"current-keys": n.trail.keys,
				sections: n.sections,
				"row-toggle": !n.sections,
				"open-trail": n.openTrail
			}, null, 8, [
				"items",
				"current-keys",
				"sections",
				"row-toggle",
				"open-trail"
			])]),
			K("div", ql, [
				n.actions.length > 0 ? (U(), W("div", Jl, [(U(!0), W(V, null, z(n.actions, (e) => {
					var t, n;
					return U(), G(P(Us), {
						key: e.key,
						variant: (t = e.variant) == null ? "primary" : t,
						href: (n = e.url) == null ? "" : n,
						onClick: (t) => r("action", e)
					}, {
						default: L(() => [J(T(e.title), 1)]),
						_: 2
					}, 1032, [
						"variant",
						"href",
						"onClick"
					]);
				}), 128))])) : Y("", !0),
				n.language ? (U(), W("a", {
					key: 1,
					class: "ik-header-sheet__lang",
					href: n.language.url,
					"aria-label": n.language.title
				}, [K("span", Xl, T(n.language.current), 1), K("span", Zl, T(n.language.other), 1)], 8, Yl)) : Y("", !0),
				n.social.length > 0 ? (U(), W("ul", Ql, [(U(!0), W(V, null, z(n.social, (e) => (U(), W("li", { key: e.key }, [K("a", {
					href: e.url,
					target: e.target,
					rel: e.rel,
					title: e.title
				}, [e.icon ? (U(), G(P(ic), {
					key: 0,
					icon: e.icon
				}, null, 8, ["icon"])) : (U(), W("span", eu, T(e.title), 1))], 8, $l)]))), 128))])) : Y("", !0)
			]),
			K("span", {
				"data-ik-sentinel": "",
				tabindex: "0",
				onFocus: t[2] || (t[2] = (e) => p("end"))
			}, null, 32)
		], 40, Gl)], 64)) : Y("", !0)], 2));
	}
}), ru = { class: "ik-header__row1" }, iu = { class: "ik-header__row1-inner" }, au = { class: "ik-header__left" }, ou = {
	key: 1,
	class: "ik-header__nav"
}, su = [
	"href",
	"target",
	"rel",
	"aria-current"
], cu = { class: "ik-header__right" }, lu = {
	key: 0,
	class: "ik-header__actions"
}, uu = ["href", "aria-label"], du = /* @__PURE__ */ R({
	__name: "SiteHeader",
	props: {
		data: {},
		sheetSections: {
			type: Boolean,
			default: !0
		},
		sheetOpenTrail: {
			type: Boolean,
			default: !0
		},
		breakpoint: { default: "l" }
	},
	setup(e) {
		let t = e, n = yo();
		function r(e) {
			e.event !== void 0 && e.event !== "" && n(e.event, { key: e.key });
		}
		return (e, n) => (U(), W("div", { class: w(["ik-root ik-header", { "ik-header--xl": t.breakpoint === "xl" }]) }, [K("div", ru, [K("div", iu, [K("div", au, [t.data.logo === null ? Y("", !0) : (U(), G($n(t.data.logo === null || t.data.logo.url === "" ? "div" : "a"), {
			key: 0,
			class: "ik-header__logo",
			href: t.data.logo.url === "" ? void 0 : t.data.logo.url,
			"aria-label": t.data.logo.title
		}, {
			default: L(() => [t.data.logo.icon ? (U(), G(P(ic), {
				key: 0,
				icon: t.data.logo.icon
			}, null, 8, ["icon"])) : Y("", !0)]),
			_: 1
		}, 8, ["href", "aria-label"])), t.data.menu.length > 0 ? (U(), W("ul", ou, [(U(!0), W(V, null, z(t.data.menu, (e) => {
			var n;
			return U(), W("li", {
				key: e.key,
				class: w(["ik-header__nav-item", { "ik-header__nav-item--current": t.data.trail.keys.has(e.key) }])
			}, [e.items.length > 0 ? (U(), G(P(dc), {
				key: 0,
				label: e.title,
				items: e.items,
				layout: (n = e.panel) == null ? "list" : n,
				columns: e.columns,
				current: t.data.trail.keys.has(e.key)
			}, null, 8, [
				"label",
				"items",
				"layout",
				"columns",
				"current"
			])) : (U(), W("a", {
				key: 1,
				class: "ik-header__nav-link",
				href: e.url,
				target: e.target,
				rel: e.rel,
				"aria-current": t.data.trail.activeKey === e.key ? t.data.trail.matched === "path" ? "page" : "location" : void 0
			}, T(e.title), 9, su))], 2);
		}), 128))])) : Y("", !0)]), K("div", cu, [
			t.data.actions.length > 0 ? (U(), W("div", lu, [(U(!0), W(V, null, z(t.data.actions, (e) => {
				var t, n;
				return U(), G(P(Us), {
					key: e.key,
					variant: (t = e.variant) == null ? "primary" : t,
					href: (n = e.url) == null ? "" : n,
					onClick: (t) => r(e)
				}, {
					default: L(() => [J(T(e.title), 1)]),
					_: 2
				}, 1032, [
					"variant",
					"href",
					"onClick"
				]);
			}), 128))])) : Y("", !0),
			t.data.language ? (U(), W("a", {
				key: 1,
				class: "ik-header__lang",
				href: t.data.language.url,
				"aria-label": t.data.language.title
			}, [K("span", null, T(t.data.language.current), 1), K("span", null, T(t.data.language.other), 1)], 8, uu)) : Y("", !0),
			nr(e.$slots, "aside"),
			q(nu, {
				menu: t.data.menu,
				social: t.data.social,
				actions: t.data.actions,
				language: t.data.language,
				trail: t.data.trail,
				sections: t.sheetSections,
				"open-trail": t.sheetOpenTrail,
				breakpoint: t.breakpoint,
				onAction: r
			}, null, 8, [
				"menu",
				"social",
				"actions",
				"language",
				"trail",
				"sections",
				"open-trail",
				"breakpoint"
			])
		])])]), t.data.product === null ? Y("", !0) : (U(), G(Hl, {
			key: 0,
			product: t.data.product,
			trail: t.data.trail,
			breakpoint: t.breakpoint
		}, null, 8, [
			"product",
			"trail",
			"breakpoint"
		]))], 2));
	}
});
//#endregion
//#region src/app/islands/site-header.ts
function fu(e) {
	return $(e).toLowerCase() === "xl" ? "xl" : "l";
}
var pu = "ik-l1-", mu = "ik-l2-";
function hu(e) {
	let t = X(e);
	if (t === null) return null;
	let n = $(Q(t, "title", "name", "alt")), r = tl(Q(t, "icon", "image", "src"), n);
	return r === void 0 && n === "" ? null : {
		title: n,
		url: $(Q(t, "url", "href")),
		...r ? { icon: r } : {}
	};
}
function gu(e) {
	let t = X(e);
	if (t === null) return;
	let n = $(Q(t, "url", "href")), r = $(Q(t, "current")), i = $(Q(t, "other"));
	if (n !== "" && r !== "" && i !== "") return {
		url: n,
		current: r,
		other: i,
		title: $(Q(t, "title", "label")) || i
	};
}
function _u(e) {
	let t = [];
	return Z(e).forEach((e, n) => {
		let r = X(e);
		if (r === null) return;
		let i = $(Q(r, "title", "label", "text"));
		if (i === "") return;
		let a = $(Q(r, "url", "href")), o = $(Q(r, "event")), s = $(Q(r, "variant")).toLowerCase();
		t.push({
			key: $(Q(r, "key", "id")) || `action-${n}`,
			title: i,
			...a === "" ? {} : { url: a },
			...o === "" ? {} : { event: o },
			...s === "primary" || s === "secondary" || s === "ghost" ? { variant: s } : {}
		});
	}), t;
}
function vu(e) {
	let t = $(Q(e, "currentPath", "current_path", "path"));
	return t === "" ? typeof window > "u" ? "" : window.location.pathname : t;
}
function yu(e, t) {
	let n = /* @__PURE__ */ new Set([...e.keys, ...t.keys]), r = (e) => e.matched === "path" ? 3 : e.matched === "section" ? 2 : +(e.matched === "flag"), i = r(e) > r(t) ? e : t;
	return {
		keys: n,
		activeKey: i.activeKey,
		matched: i.matched
	};
}
//#endregion
//#region src/app/standalone.ts
Oo([
	el,
	Dl,
	To({
		id: "site-header",
		kind: "content",
		component: du,
		parseProps: (e) => {
			let t = vu(e), n = vl(Q(e, "menu", "nav", "items"), { keyPrefix: pu }), r = vl(Q(e, "productMenu", "product_menu", "products"), { keyPrefix: mu }), i = vl(Q(e, "social", "socialLinks", "social_links"), { keyPrefix: "ik-social-" });
			n.dropped + r.dropped + i.dropped;
			let a = $(Q(e, "productPath", "product_path")), o = El(r.items, a === "" ? t : a), s = gu(Q(e, "language", "languageSwitch", "language_switch"));
			return {
				data: {
					logo: hu(Q(e, "logo")),
					menu: n.items,
					product: o,
					actions: _u(Q(e, "actions", "buttons")),
					...s === void 0 ? {} : { language: s },
					social: i.items,
					trail: yu(Tl(n.items, t), Tl(o === null ? [] : o.items, t))
				},
				sheetSections: !["accordion", "collapsed"].includes($(Q(e, "sheet"))),
				sheetOpenTrail: $(Q(e, "sheet")) !== "collapsed",
				breakpoint: fu(Q(e, "breakpoint"))
			};
		},
		messageKeys: []
	})
]);
var bu = {
	version: "0.11.0",
	autoMount: es,
	mountIsland: Go,
	registeredIslandIds: Ao
};
window.islandKit = bu, es({ context: Zo() });
//#endregion
export { es as autoMount, bu as default, Go as mountIsland, Ao as registeredIslandIds };
