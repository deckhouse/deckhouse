async function initializeChannelMenu() {
    try {
        const yamlText = await loadYAMLFile('/includes/release-channels/channels.yaml');
        const appData = jsyaml.load(yamlText);

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
                const stabilityOrder = { 'rock-solid': 5, 'stable': 4, 'ea': 3, 'beta': 2, 'alpha': 1, 'latest': 0 };
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

    // Format the channel name for display
    const formattedChannel = currentChannel
        .replace(/ea/g, 'early access')
        .replace(/early-access/g, 'Early Access')
        .replace(/-/g, ' ')
        .replace(/\b\w/g, l => l.toUpperCase());

    currentVersionElement.textContent = formattedChannel;
}

// Use the JSON structure in your logic
function renderMenu(settings) {
    const menuContainer = document.getElementById('doc-versions-menu');
    // Check page type from meta tag
    const pageTypeMeta = document.querySelector('meta[name="page:module:type"]');
    // isFromSource is false for embedded modules, true for modules from source
    const isFromSource = pageTypeMeta && pageTypeMeta.getAttribute('content') === 'from-source';

    if (!menuContainer) {
        console.error('Channel menu container not found');
        return;
    }

    // Check if settings has channels data
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
        const stabilityOrder = { 'rock-solid': 5, 'stable': 4, 'ea': 3, 'beta': 2, 'alpha': 1, 'latest': 0 };
        const aStability = stabilityOrder[a.name] || 0;
        const bStability = stabilityOrder[b.name] || 0;
        return bStability - aStability;
    });

    if (!isFromSource) {
        // For embedded modules, add latest channel to the end of the channel list
        sortedChannels.push({ name: 'latest', version: 'latest' });
    }

    // Create submenu container
    const submenuContainer = document.createElement('div');
    submenuContainer.className = 'submenu-container';

    // Create submenu list
    const submenu = document.createElement('ul');
    submenu.className = 'submenu';

    // Iterate through sorted channels and create menu items
    sortedChannels.forEach((channel, index) => {
        const submenuItem = document.createElement('li');
        submenuItem.className = 'submenu-item';

        const submenuItemLink = document.createElement('a');

        // Generate channel URL according to the rules
        let channelUrl = '#';

        if (channel.version) {
            const currentUrl = window.location.pathname;

            if (isFromSource) {
                // For modules from source use channel name instead of channel version in the link
                const channelName = channel.name;
                if (currentUrl.match(/\/modules\/[^\/]+\/(alpha|beta|early-access|stable|rock-solid|latest)\//)) {
                    // Current URL has channel, replace it
                    console.log("Current URL has channel, replace it: ", currentUrl);
                    channelUrl = currentUrl.replace(/\/(alpha|beta|early-access|stable|rock-solid|latest)\//, `/${channelName}/`);
                } else if (currentUrl.includes('/modules/')) {
                    // Current URL is /modules/MODULE/, add channel name
                    console.log("Current URL format - /modules/MODULE/:", currentUrl);
                    channelUrl = currentUrl.replace(/\/modules\/([^/]+)\//, `/modules/$1/${channelName}/`);
                }
            } else {
                // For embedded modules, use channel version in the link
                const urlVersion = `${channel.version}`;
                if (currentUrl.match(/\/modules\/[^\/]+\/(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest|)\//)) {
                    // Current URL has version, replace it with channel version
                    channelUrl = currentUrl.replace(/\/(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest)\//, `/${urlVersion}/`);
                } else if (currentUrl.includes('/modules/')) {
                    // Current URL is /modules/MODULE/, add version
                    channelUrl = currentUrl.replace(/\/modules\/([^/]+)\//, `/modules/$1/${urlVersion}/`);
                }
            }
        } else {
            // No version available, use current URL
            channelUrl = window.location.pathname;
        }

        submenuItemLink.href = channelUrl;
        submenuItemLink.className = 'submenu-item-link';
        submenuItemLink.setAttribute('data-proofer-ignore', '');

        // Create channel name span - replace dashes with spaces and capitalize
        const channelSpan = document.createElement('span');
        channelSpan.className = 'submenu-item-channel';
        const formattedName = (channel.name || 'Unknown Channel')
            .replace(/ea/g, 'early access')
            .replace(/-/g, ' ')
            .replace(/\b\w/g, l => l.toUpperCase());
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

        // Assemble the link
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
