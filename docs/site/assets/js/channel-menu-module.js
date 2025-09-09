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
        showError('Failed to load application data');
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

// Get current channel from URL and update the doc-current-version element
function updateCurrentVersion() {
    const currentVersionElement = document.getElementById('doc-current-version');
    if (!currentVersionElement) {
        console.warn('doc-current-version element not found');
        return;
    }

    // Extract channel from URL pattern /modules/CHANNEL/...
    const urlPath = window.location.pathname;
    const channelMatch = urlPath.match(/\/modules\/([^\/]+)\/(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest|)\//);

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
    if (window.channelsData && extractedChannel) {
        const matchingChannels = window.channelsData.filter(channel => {
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
            const currentUrl = window.location.pathname;

            if (isEmbeddedModule) {
                // For embedded modules, use channel version in the link
                const urlVersion = `${channel.version}`;
                if (currentUrl.match(/\/modules\/[^\/]+\/(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest|)\//)) {
                    // Current URL has version, replace it with channel version
                    channelUrl = currentUrl.replace(/\/(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest)\//, `/${urlVersion}/`);
                } else if (currentUrl.includes('/modules/')) {
                    // Current URL is /modules/MODULE/, add version
                    channelUrl = currentUrl.replace(/\/modules\/([^/]+)\//, `/modules/$1/${urlVersion}/`);
                }
            } else {
                // For modules from source use channel name instead of channel version in the link
                const channelName = channel.name;
                if (currentUrl.match(/\/modules\/[^\/]+\/(alpha|beta|early-access|stable|rock-solid|latest)\//)) {
                    // Current URL has channel, replace it
                    channelUrl = currentUrl.replace(/\/(alpha|beta|early-access|stable|rock-solid|latest)\//, `/${channelName}/`);
                } else if (currentUrl.includes('/modules/')) {
                    // Current URL is /modules/MODULE/, add channel name
                    channelUrl = currentUrl.replace(/\/modules\/([^/]+)\//, `/modules/$1/${channelName}/`);
                }
            }
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

    submenuContainer.appendChild(submenu);
    menuContainer.appendChild(submenuContainer);
}

// Initialize the app when the page loads
document.addEventListener('DOMContentLoaded', initializeChannelMenu);
