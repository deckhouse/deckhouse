async function initializeChannelMenu() {
    try {
        const yamlText = await loadYAMLFile('/includes/release-channels/channels.yaml');
        const appData = jsyaml.load(yamlText);

        renderMenu(appData);
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

// Use the JSON structure in your logic
function renderMenu(settings) {
    console.debug('Channels data:', settings);
}

// Initialize the app when the page loads
document.addEventListener('DOMContentLoaded', initializeChannelMenu);
