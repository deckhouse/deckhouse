async function initializeChannelMenu() {
    try {
        const yamlText = await loadYAMLFile('/includes/release-channels/channels.yaml');
        const appData = jsyaml.load(yamlText);

        // Replace 'ea' with 'early-access' when loading data
        if (appData.groups[0].channels) {
            appData.groups[0].channels = appData.groups[0].channels.map(channel => {
                if (channel.name === 'ea') {
                    return { ...channel, name: 'early-access' };
                }
                return channel;
            });
        }

        // Store channels data globally for use in updateCurrentVersion
        window.channelsData = appData.groups[0].channels;

        renderMenu(appData.groups[0]);
        updateCurrentVersion();
    } catch (error) {
        console.error('Failed to initialize app:', error);
        updateCurrentVersion();
    }
}

// Load YAML file as text
async function loadYAMLFile(url) {
    const response = await fetch(url);
    if (!response.ok) {
        throw new Error(`Failed to load ${url}: ${response.status}`);
    }
    return await response.text();
}

// Format channel name for display (e.g., "early-access" -> "Early Access")
function formatChannelName(channelName) {
    if (!channelName) {
        return 'Unknown Channel';
    }
    return channelName
        .replace(/-/g, ' ')
        .replace(/\b\w/g, l => l.toUpperCase());
}

// The version or channel of a module address: /modules/MODULE/SEGMENT/.
const versionSegment = /\/modules\/([^\/]+)\/(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest)\//;

// The address of the same page in another version or channel.
function versionHref(path, value) {
    if (versionSegment.test(path)) {
        return path.replace(versionSegment, `/modules/$1/${value}/`);
    }
    return path.replace(/\/modules\/([^/]+)\//, `/modules/$1/${value}/`);
}

// Get current channel from URL and update the doc-current-version element
function updateCurrentVersion() {
    const currentVersionElement = document.getElementById('doc-current-version');
    if (!currentVersionElement) {
        console.warn('doc-current-version element not found');
        return;
    }

    // Extract channel from URL pattern /modules/CHANNEL/...
    const urlPath = window.location.pathname;
    const channelMatch = urlPath.match(versionSegment);

    let currentChannel = 'stable'; // default fallback
    let extractedChannel = null;

    if (channelMatch) {
        extractedChannel = channelMatch[2];
        // Check if extracted channel is one of the valid channels
        const validChannels = ['alpha', 'beta', 'early-access', 'stable', 'rock-solid', 'latest'];
        if (validChannels.includes(extractedChannel)) {
            currentChannel = extractedChannel;
        }
    }

    // Try to find the actual channel name from the channels data
    // This will be called after the channels data is loaded
    if (extractedChannel) {
        const matchingChannels = (window.channelsData || []).filter(channel => {
            // Check if the channel name or version matches the extracted channel
            return channel.name === extractedChannel || channel.version === extractedChannel;
        });

        if (matchingChannels.length > 0) {
            // If multiple channels have the same version, find the most stable one
            if (matchingChannels.length > 1) {
                const stabilityOrder = { 'rock-solid': 5, 'stable': 4, 'early-access': 3, 'beta': 2, 'alpha': 1, 'latest': 0 };
                const mostStableChannel = matchingChannels.reduce((prev, current) => {
                    const prevStability = stabilityOrder[prev.name] || 0;
                    const currentStability = stabilityOrder[current.name] || 0;
                    return currentStability > prevStability ? current : prev;
                });
                currentChannel = mostStableChannel.name;
            } else {
                currentChannel = matchingChannels[0].name;
            }
        } else if (/^v[0-9]/.test(extractedChannel)) {
            // A version no channel points to, picked in the versions dialog.
            currentVersionElement.textContent = extractedChannel;
            return;
        }
    }

    const formattedChannel = formatChannelName(currentChannel);
    currentVersionElement.textContent = formattedChannel;
}

function renderMenu(settings) {
    const menuContainer = document.getElementById('doc-versions-menu');
    const pageModuleType = document.querySelector('meta[name="page:module:type"]');
    // isEmbeddedModule is true for embedded modules, false for modules from source
    const isEmbeddedModule = pageModuleType && pageModuleType.getAttribute('content') === 'embedded';

    if (!menuContainer) {
        console.error('Channel menu container not found');
        return;
    }

    if (!settings || !settings.channels) {
        console.warn('No channels data found in settings');
        return;
    }

    // Find existing submenu-container and remove it
    const existingSubmenu = menuContainer.querySelector('.submenu-container');
    if (existingSubmenu) {
        existingSubmenu.remove();
    }

    // Sort channels by stability in descending order (rock-solid first)
    const sortedChannels = [...settings.channels].sort((a, b) => {
        const stabilityOrder = { 'rock-solid': 5, 'stable': 4, 'early-access': 3, 'beta': 2, 'alpha': 1, 'latest': 0 };
        const aStability = stabilityOrder[a.name] || 0;
        const bStability = stabilityOrder[b.name] || 0;
        return bStability - aStability;
    });

    if (isEmbeddedModule) {
        // For embedded modules, add latest channel to the end of the channel list
        sortedChannels.push({ name: 'latest', version: 'latest' });
    }

    // Create submenu container
    const submenuContainer = document.createElement('div');
    submenuContainer.className = 'submenu-container';

    const submenu = document.createElement('ul');
    submenu.className = 'submenu';

    // Create menu items
    sortedChannels.forEach((channel, index) => {
        const submenuItem = document.createElement('li');
        submenuItem.className = 'submenu-item';

        const submenuItemLink = document.createElement('a');

        let channelUrl = '#';

        if (channel.version) {
            // Embedded modules link the version of the channel, modules from
            // sources the channel itself.
            channelUrl = versionHref(window.location.pathname, isEmbeddedModule ? channel.version : channel.name);
        } else {
            // No version available, use current URL
            channelUrl = window.location.pathname;
            console.warn("Channel version not specified");
        }

        submenuItemLink.href = channelUrl;
        submenuItemLink.className = 'submenu-item-link';
        submenuItemLink.setAttribute('data-proofer-ignore', '');

        // Create channel name span
        const channelSpan = document.createElement('span');
        channelSpan.className = 'submenu-item-channel';
        const formattedName = formatChannelName(channel.name);
        channelSpan.textContent = formattedName;

        // Create dot separator - use special class if same release version as previous item
        const dotSpan = document.createElement('span');
        const previousChannel = index > 0 ? sortedChannels[index - 1] : null;
        const isSpecialDot = previousChannel && channel.version && channel.version === previousChannel.version;
        dotSpan.className = isSpecialDot ? 'submenu-item-dot submenu-item-dot_special' : 'submenu-item-dot';

        // Create release version span
        const releaseSpan = document.createElement('span');
        releaseSpan.className = 'submenu-item-release';
        releaseSpan.textContent = channel.version || 'latest';

        submenuItemLink.appendChild(channelSpan);
        submenuItemLink.appendChild(dotSpan);
        submenuItemLink.appendChild(releaseSpan);

        submenuItem.appendChild(submenuItemLink);
        submenu.appendChild(submenuItem);
    });

    if (isEmbeddedModule) {
        addOtherVersions(submenu);
    }

    submenuContainer.appendChild(submenu);
    menuContainer.appendChild(submenuContainer);
}

// "Show other versions" of an embedded module: the dialog of
// version-dialog.js with every minor version of Deckhouse whose documentation
// of the module the site keeps; docs-builder lists them from the history of
// the embedded modules. The texts are in version-dialog-i18n.js.
function addOtherVersions(submenu) {
    const moduleMatch = window.location.pathname.match(/\/modules\/([^\/]+)\//);
    const lang = (document.documentElement.lang || '').startsWith('ru') ? 'ru' : 'en';
    const t = (window.versionDialogTexts || {})[lang];
    if (!t || typeof window.createVersionDialog !== 'function') {
        console.warn('versions dialog: version-dialog-i18n.js or version-dialog.js is not loaded');
    }
    if (!moduleMatch || !t || typeof window.createVersionDialog !== 'function') {
        return;
    }
    const module = moduleMatch[1];

    // The version of the page: in the address, or that of its channel.
    const segment = (window.location.pathname.match(versionSegment) || [])[2] || 'stable';
    const channels = window.channelsData || [];
    const channelOf = name => channels.find(channel => channel.name === name);
    const shownVersion = /^v[0-9]/.test(segment) ? segment : ((channelOf(segment) || {}).version || segment);

    const dialog = window.createVersionDialog({
        module,
        texts: t,
        shownVersion,
        embedded: true,
        href: version => versionHref(window.location.pathname, version),
        load: () => fetch(`/modules/.versions/${module}`, { headers: { Accept: 'application/json' } })
            // 404: no minor version kept the module yet, latest only.
            .then(response => (response.ok ? response.json() : response.status === 404 ? { versions: [] } : Promise.reject(response.status))),
        badges: version => channels.filter(channel => channel.version === version)
            .map(channel => [formatChannelName(channel.name), '']),
    });
    if (!dialog) {
        return;
    }

    const other = document.createElement('li');
    other.className = 'vmenu-other';
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'vmenu-other__button';
    button.textContent = t.other;
    button.addEventListener('click', () => dialog.open());
    other.append(button);
    submenu.append(other);
}

// Initialize the app when the page loads
document.addEventListener('DOMContentLoaded', initializeChannelMenu);
