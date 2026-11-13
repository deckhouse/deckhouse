// The channel and version menu of a page of a module from sources
// (module-version-badge.html of docs-builder-template). One rendered version
// serves the address of the version and of every channel pointing at it, so
// the menu is built here from what docs-builder serves now:
//   - the public site: the release channels with their versions;
//   - a cluster: the installed version, or that of the stable channel of a
//     disabled module.
// "Show other versions" opens the dialog of version-dialog.js with every
// published version of the module. The page carries the configuration in
// <script data-vmenu-config>, the styles are in module-versions.css.
(function () {
    const configNode = document.querySelector('[data-vmenu-config]');
    const label = document.querySelector('[data-vmenu-label]');
    const items = document.querySelector('[data-vmenu-items]');
    if (!configNode || !label || !items) {
        return;
    }
    const config = JSON.parse(configNode.textContent);
    const t = config.texts;

    // The page of the address after the module and its version or channel:
    // the menu leads to the same page of another version.
    const moduleBase = '/modules/' + config.module + '/';
    const path = window.location.pathname;
    let rest = path.slice(path.indexOf(moduleBase) + moduleBase.length);
    const segment = rest.split('/')[0];
    const channelCodes = config.channels.map(c => c.code);
    const isVersion = /^v\d+\.\d+(\.\d+.*)?$/.test(segment) || segment === 'latest';
    const shownSegment = (isVersion || channelCodes.includes(segment)) && rest.includes('/') ? segment : '';
    if (shownSegment) {
        rest = rest.slice(shownSegment.length + 1);
    }
    const href = target => config.prefix + moduleBase + (target ? target + '/' : '') + rest;

    const other = items.querySelector('.vmenu-other');
    const channelTitle = code => (config.channels.find(c => c.code === code) || {}).title || code;

    label.addEventListener('click', event => event.preventDefault());
    if (channelCodes.includes(shownSegment)) {
        label.textContent = channelTitle(shownSegment);
    } else if (!shownSegment && !config.cluster) {
        label.textContent = channelTitle('stable');
    }

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
    const menuItem = (title, version, link) => {
        const li = el('li', 'submenu-item');
        const a = el('a', 'submenu-item-link');
        a.href = link;
        a.setAttribute('data-proofer-ignore', '');
        a.append(el('span', 'submenu-item-channel', title), el('span', 'submenu-item-dot'), el('span', 'submenu-item-release', version));
        li.append(a);
        return li;
    };

    // A module on the list whose channels are not linked yet (a first pass on
    // an empty cache): the menu says the versions come later.
    const pending = () => items.insertBefore(el('li', 'submenu-caption', t.pending), other);
    let menu = { enabled: false, channels: {} };
    const menuLoaded = fetch(config.menuUrl, { headers: { Accept: 'application/json' } })
        .then(response => (response.ok ? response.json() : Promise.reject(response.status)))
        .then(data => {
            menu = { enabled: !!data.enabled, channels: data.channels || {} };
            if (config.cluster) {
                if (menu.channels.stable) {
                    items.insertBefore(menuItem(menu.enabled ? t.installed : channelTitle('stable'), menu.channels.stable, href('')), other);
                } else {
                    pending();
                }
                return;
            }
            let shown = 0;
            config.channels.forEach(channel => {
                const version = menu.channels[channel.code];
                if (version) {
                    items.insertBefore(menuItem(channel.title, version, href(channel.code)), other);
                    shown++;
                }
            });
            if (shown === 0) {
                pending();
            }
        })
        // Without the menu the versions dialog still works, unmarked.
        .catch(error => console.warn('version menu of the module: ' + error));
    menuLoaded.then(() => {
        if (config.embedded.length === 0) {
            return;
        }
        const list = el('li', 'submenu-embedded-list');
        config.embedded.forEach(productVersion => {
            const a = el('a', 'submenu-embedded-link', productVersion);
            a.href = href(productVersion);
            list.append(a);
        });
        items.insertBefore(el('li', 'submenu-caption', t.embedded), other);
        items.insertBefore(list, other);
    });

    // The dialog of all the versions.
    const open = items.querySelector('[data-vmenu-open]');
    if (!open || typeof window.createVersionDialog !== 'function') {
        return;
    }
    const dialog = window.createVersionDialog({
        module: config.module,
        texts: t,
        shownVersion: config.version,
        rel: 'nofollow',
        href,
        load: () => Promise.all([
            fetch(config.versionsUrl, { headers: { Accept: 'application/json' } })
                .then(response => (response.ok ? response.json() : Promise.reject(response.status))),
            menuLoaded,
        ]).then(([data]) => data),
        badges: version => {
            const badges = [];
            if (config.cluster && menu.enabled && menu.channels.stable === version) {
                badges.push([t.installedBadge, 'installed']);
            }
            if (!config.cluster) {
                config.channels.forEach(channel => {
                    if (menu.channels[channel.code] === version) {
                        badges.push([channel.title, '']);
                    }
                });
            }
            return badges;
        },
    });
    if (dialog) {
        open.addEventListener('click', () => dialog.open());
    } else {
        other.remove();
    }
})();
