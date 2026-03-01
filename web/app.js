document.addEventListener('DOMContentLoaded', () => {
    initApp();
});

const UI = {
    dateLine: document.getElementById('current-date'),
    bridgeWarning: document.getElementById('bridge-warning'),
    addServerForm: document.getElementById('add-server-form'),
    serversList: document.getElementById('servers-list'),
    clipsGrid: document.getElementById('clips-grid'),
    notificationToast: document.getElementById('notification-toast'),
    toastMessage: document.getElementById('toast-message'),
};

async function initApp() {
    // Set date
    const now = new Date();
    const options = { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' };
    UI.dateLine.textContent = now.toLocaleDateString('en-US', options);

    // Check Bridge
    if (!window.Bridge) {
        UI.bridgeWarning.classList.remove('hidden');
        console.error('Bridge API not found. Please open in Clip Dock.');
        return;
    }

    // Event Listeners
    UI.addServerForm.addEventListener('submit', handleAddServer);

    // Initial Load
    await refreshData();
}

async function refreshData() {
    await Promise.all([
        loadServers(),
        loadClips()
    ]);
}

async function invoke(command, stdin = '{}') {
    try {
        const result = await window.Bridge.invoke(command, stdin);
        if (result.exitCode !== 0) {
            showNotification(`Error: ${result.stderr || 'Unknown error'}`);
            return null;
        }
        return JSON.parse(result.stdout);
    } catch (error) {
        console.error(`Failed to invoke ${command}:`, error);
        showNotification(`Failed to run ${command}`);
        return null;
    }
}

async function loadServers() {
    const data = await invoke('list-servers');
    if (!data) return;

    renderServers(data.servers || []);
}

async function loadClips() {
    const data = await invoke('list');
    if (!data) return;

    renderClips(data.clips || []);
}

function renderServers(servers) {
    if (servers.length === 0) {
        UI.serversList.innerHTML = '<p class="empty-state">No servers configured.</p>';
        return;
    }

    UI.serversList.innerHTML = servers.map(server => `
        <div class="server-item">
            <div class="server-info">
                <h4>${server.name}</h4>
                <p>${server.server_url}</p>
                <p>Token hint: ${server.token_hint || '****'}</p>
            </div>
            <button class="btn btn-outline btn-small" onclick="handleRemoveServer('${server.name}')">Remove</button>
        </div>
    `).join('');
}

function renderClips(clips) {
    if (clips.length === 0) {
        UI.clipsGrid.innerHTML = '<p class="empty-state">No clips found.</p>';
        return;
    }

    UI.clipsGrid.innerHTML = clips.map(clip => `
        <article class="clip-card">
            <h3>${clip.name}</h3>
            <p class="clip-desc">${clip.desc || 'No description available.'}</p>
            
            <div class="tag-list">
                ${clip.commands.map(cmd => `<span class="tag">${cmd}</span>`).join('')}
            </div>

            <div class="clip-meta">
                ${clip.hasWeb ? '<span class="badge">Web Ready</span>' : ''}
                <span class="badge">${clip.server}</span>
            </div>

            <button class="btn btn-outline" style="margin-top: 1rem;" 
                onclick="handleGenerateBookmark('${clip.server}', '${clip.clipId}')">
                Add to Dock
            </button>
        </article>
    `).join('');
}

async function handleAddServer(e) {
    e.preventDefault();
    
    const name = document.getElementById('server-name').value;
    const server_url = document.getElementById('server-url').value;
    const token = document.getElementById('server-token').value;

    const stdin = JSON.stringify({ name, server_url, token });
    const result = await invoke('add-server', stdin);

    if (result && result.ok) {
        showNotification(`Server ${name} added successfully.`);
        UI.addServerForm.reset();
        await refreshData();
    }
}

async function handleRemoveServer(name) {
    if (!confirm(`Are you sure you want to remove server "${name}"?`)) return;

    const stdin = JSON.stringify({ name });
    const result = await invoke('remove-server', stdin);

    if (result && result.ok) {
        showNotification(`Server ${name} removed.`);
        await refreshData();
    }
}

async function handleGenerateBookmark(server, clip_id) {
    const stdin = JSON.stringify({ server, clip_id });
    const result = await invoke('generate-bookmark', stdin);

    if (result) {
        const json = JSON.stringify(result, null, 2);
        try {
            await navigator.clipboard.writeText(json);
            showNotification('Copied to clipboard!');
        } catch (err) {
            console.error('Failed to copy:', err);
            // Fallback: show the JSON in an alert or something if clipboard fails
            showNotification('Failed to copy to clipboard. Check console.');
            console.log(json);
        }
    }
}

let toastTimeout;
function showNotification(message) {
    UI.toastMessage.textContent = message;
    UI.notificationToast.classList.remove('hidden');

    clearTimeout(toastTimeout);
    toastTimeout = setTimeout(() => {
        UI.notificationToast.classList.add('hidden');
    }, 3000);
}

// Make globally available for onclick handlers
window.handleRemoveServer = handleRemoveServer;
window.handleGenerateBookmark = handleGenerateBookmark;
