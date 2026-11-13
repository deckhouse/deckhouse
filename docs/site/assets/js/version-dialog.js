// The dialog with every published version of a module: a filter, pages and
// badges next to the versions. One dialog for the pages of the modules from
// sources (module-version-menu.js) and of the embedded modules
// (channel-menu-module.js); the styles are in module-versions.css.
//
// createVersionDialog({
//   module,          the name of the module, for the title
//   texts,           title (### stands for the module), close, hint, filter,
//                    prev, next, page, count, empty, none, loading, failed,
//                    incomplete, shown; page and count take {page}, {pages}
//                    and {count}
//   href(version),   the address of the same page in the version
//   load(),          a promise of the versions, the newest first, or of
//                    { versions, incomplete }: incomplete marks a list that
//                    may miss versions, the dialog says so
//   badges(version), [[text, kind]] shown after the version; kind is a
//                    modifier of .vmenu-dialog__badge or ''
//   shownVersion,    the version the page shows: marked in the list
//   rel,             the rel of the links, e.g. 'nofollow'
//   embedded,        marks the dialog of an embedded module, whose links
//                    doc-version-link-resolver.js checks as those of the menu
// }) returns { open() }, or null when the browser has no <dialog>.
window.createVersionDialog = function (options) {
    if (typeof HTMLDialogElement === 'undefined') {
        return null;
    }
    const t = options.texts;
    const pageSize = 15;
    const fill = (text, values) => text.replace(/\{(\w+)\}/g, (_, key) => values[key]);
    const el = (tag, className, text) => {
        const node = document.createElement(tag);
        if (className) {
            node.className = className;
        }
        if (text !== undefined) {
            node.textContent = text;
        }
        return node;
    };

    // One dialog per page: the menu may be built anew.
    document.querySelectorAll('.vmenu-dialog').forEach(old => old.remove());
    const dialog = el('dialog', 'vmenu-dialog');
    dialog.setAttribute('aria-labelledby', 'vmenu-title');
    if (options.embedded) {
        dialog.dataset.embedded = '';
    }
    const body = el('div', 'vmenu-dialog__body');
    const head = el('div', 'vmenu-dialog__head');
    const title = el('h2', 'vmenu-dialog__title', t.title.replace('###', options.module));
    title.id = 'vmenu-title';
    const close = el('button', 'vmenu-dialog__close');
    close.type = 'button';
    close.setAttribute('aria-label', t.close);
    close.innerHTML = '<svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="M1 1l12 12M13 1L1 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>';
    head.append(title, close);
    const filter = el('input', 'vmenu-dialog__filter');
    filter.type = 'search';
    filter.placeholder = t.filter;
    filter.setAttribute('aria-label', t.filter);
    filter.autocomplete = 'off';
    filter.spellcheck = false;
    const status = el('p', 'vmenu-dialog__status');
    status.setAttribute('role', 'status');
    // Shown when the registry gave a part of the versions only.
    const notice = el('p', 'vmenu-dialog__status', t.incomplete);
    notice.hidden = true;
    const list = el('ol', 'vmenu-dialog__list');
    const pager = el('div', 'vmenu-dialog__pager');
    pager.hidden = true;
    const prev = el('button', 'button button_default vmenu-dialog__page-button', t.prev);
    prev.type = 'button';
    const pageLabel = el('span', 'vmenu-dialog__page');
    const next = el('button', 'button button_default vmenu-dialog__page-button', t.next);
    next.type = 'button';
    pager.append(prev, pageLabel, next);
    body.append(head, el('p', 'vmenu-dialog__hint', t.hint), filter, status, notice, list, pager);
    dialog.append(body);
    // Out of the channel menu: it shows and hides on hover.
    document.body.append(dialog);

    let versions = null;
    let page = 0;
    const render = () => {
        const query = filter.value.trim().toLowerCase().replace(/^v/, '');
        // From the start: 1.6 finds v1.6.x, not v1.16.
        const shown = versions.filter(v => v.toLowerCase().replace(/^v/, '').startsWith(query));
        const pages = Math.max(1, Math.ceil(shown.length / pageSize));
        page = Math.min(page, pages - 1);
        list.replaceChildren(...shown.slice(page * pageSize, (page + 1) * pageSize).map(version => {
            const current = version === options.shownVersion;
            const li = el('li', 'vmenu-dialog__item' + (current ? ' vmenu-dialog__item--shown' : ''));
            const a = el('a');
            a.href = options.href(version);
            a.setAttribute('data-proofer-ignore', '');
            if (options.rel) {
                a.rel = options.rel;
            }
            const badges = el('span', 'vmenu-dialog__badges');
            const kinds = current ? [[t.shown, 'shown']] : [];
            kinds.concat(options.badges ? options.badges(version) : []).forEach(([text, kind]) => {
                badges.append(el('span', 'vmenu-dialog__badge' + (kind ? ' vmenu-dialog__badge--' + kind : ''), text));
            });
            a.append(el('span', 'vmenu-dialog__version', version), badges);
            li.append(a);
            return li;
        }));
        // No version at all is not a filter that finds nothing.
        if (versions.length === 0) {
            status.textContent = t.none || t.empty;
        } else {
            status.textContent = shown.length === 0 ? t.empty : fill(t.count, { count: shown.length });
        }
        pager.hidden = pages < 2;
        pageLabel.textContent = fill(t.page, { page: page + 1, pages });
        prev.disabled = page === 0;
        next.disabled = page >= pages - 1;
    };
    const load = () => {
        if (versions) {
            render();
            return;
        }
        status.textContent = t.loading;
        options.load()
            .then(result => {
                const data = Array.isArray(result) ? { versions: result } : result || {};
                versions = data.versions || [];
                notice.hidden = !(data.incomplete && t.incomplete);
                // The notice says to try later: the next opening asks again.
                if (data.incomplete) {
                    dialog.addEventListener('close', () => { versions = null; }, { once: true });
                }
                render();
            })
            .catch(error => {
                console.warn('versions of the module: ' + error);
                status.textContent = t.failed;
            });
    };

    close.addEventListener('click', () => dialog.close());
    // A click on the backdrop lands on the dialog itself.
    dialog.addEventListener('click', event => {
        if (event.target === dialog) {
            dialog.close();
        }
    });
    filter.addEventListener('input', () => {
        page = 0;
        if (versions) {
            render();
        }
    });
    prev.addEventListener('click', () => {
        page--;
        render();
    });
    next.addEventListener('click', () => {
        page++;
        render();
    });

    return {
        open() {
            dialog.showModal();
            filter.focus();
            load();
        },
    };
};
