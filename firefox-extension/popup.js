// ProxyRouter Firefox Extension Popup Script
// Handles UI interactions and communicates with background script

class PopupManager {
  constructor() {
    this.elements = {
      statusContent: document.getElementById('status-content'),
      toggleButton: document.getElementById('toggle-button'),
      statsSection: document.getElementById('stats-section'),
      totalProxies: document.getElementById('total-proxies'),
      aliveProxies: document.getElementById('alive-proxies'),
      refreshStats: document.getElementById('refresh-stats'),
      healthCheck: document.getElementById('health-check'),
      openOptions: document.getElementById('open-options'),
      messageContainer: document.getElementById('message-container')
    };
    
    this.init();
  }
  
  async init() {
    this.setupEventListeners();
    await this.loadInitialState();
  }
  
  setupEventListeners() {
    this.elements.toggleButton.addEventListener('click', this.handleToggle.bind(this));
    this.elements.refreshStats.addEventListener('click', this.handleRefreshStats.bind(this));
    this.elements.healthCheck.addEventListener('click', this.handleHealthCheck.bind(this));
    this.elements.openOptions.addEventListener('click', this.handleOpenOptions.bind(this));
  }
  
  async loadInitialState() {
    try {
      // Get server status
      const status = await this.sendMessage({ action: 'getStatus' });
      this.updateStatusDisplay(status);
      
      // Get proxy statistics
      const stats = await this.sendMessage({ action: 'getStats' });
      this.updateStatsDisplay(stats);
      
      // Update toggle button state based on server status
      this.updateToggleButton(status.status === 'connected');
      
    } catch (error) {
      console.error('Failed to load initial state:', error);
      this.showMessage('Failed to connect to ProxyRouter server', 'error');
    }
  }
  
  async handleToggle() {
    try {
      this.elements.toggleButton.disabled = true;
      this.elements.toggleButton.textContent = 'Toggling...';
      
      const response = await this.sendMessage({ action: 'toggleProxy' });
      
      if (response.success) {
        this.updateToggleButton(response.enabled);
        this.showMessage(
          response.enabled ? 'Proxy enabled' : 'Proxy disabled', 
          'success'
        );
      } else {
        this.showMessage('Failed to toggle proxy', 'error');
      }
    } catch (error) {
      console.error('Toggle failed:', error);
      this.showMessage('Failed to toggle proxy', 'error');
    } finally {
      this.elements.toggleButton.disabled = false;
    }
  }
  
  async handleRefreshStats() {
    try {
      this.elements.refreshStats.disabled = true;
      this.elements.refreshStats.textContent = 'Refreshing...';
      
      const stats = await this.sendMessage({ action: 'getStats' });
      this.updateStatsDisplay(stats);
      
      if (stats.status === 'success') {
        this.showMessage('Statistics updated', 'success');
      } else {
        this.showMessage(stats.message || 'Failed to refresh stats', 'error');
      }
    } catch (error) {
      console.error('Refresh stats failed:', error);
      this.showMessage('Failed to refresh statistics', 'error');
    } finally {
      this.elements.refreshStats.disabled = false;
      this.elements.refreshStats.textContent = 'Refresh Stats';
    }
  }
  
  async handleHealthCheck() {
    try {
      this.elements.healthCheck.disabled = true;
      this.elements.healthCheck.textContent = 'Checking...';
      
      const result = await this.sendMessage({ action: 'triggerHealthCheck' });
      
      if (result.status === 'success') {
        this.showMessage('Health check completed successfully', 'success');
        // Refresh stats after health check
        setTimeout(() => this.handleRefreshStats(), 1000);
      } else {
        this.showMessage(result.message || 'Health check failed', 'error');
      }
    } catch (error) {
      console.error('Health check failed:', error);
      this.showMessage('Health check failed', 'error');
    } finally {
      this.elements.healthCheck.disabled = false;
      this.elements.healthCheck.textContent = 'Health Check';
    }
  }
  
  handleOpenOptions() {
    browser.runtime.openOptionsPage();
  }
  
  updateStatusDisplay(status) {
    const statusContent = this.elements.statusContent;
    
    let statusClass = 'status-error';
    let statusText = 'Disconnected';
    
    if (status.status === 'connected') {
      statusClass = 'status-connected';
      statusText = 'Connected';
      this.elements.statsSection.style.display = 'block';
    } else if (status.status === 'error') {
      statusClass = 'status-error';
      statusText = 'Error';
      this.elements.statsSection.style.display = 'none';
    }
    
    statusContent.innerHTML = `
      <span class="status-indicator ${statusClass}"></span>
      <strong>${statusText}</strong>
      <br>
      <small>${status.message || 'Unknown status'}</small>
    `;
  }
  
  updateStatsDisplay(stats) {
    if (stats.status === 'success') {
      this.elements.totalProxies.textContent = stats.totalProxies;
      this.elements.aliveProxies.textContent = stats.aliveProxies;
    } else {
      this.elements.totalProxies.textContent = '0';
      this.elements.aliveProxies.textContent = '0';
    }
  }
  
  updateToggleButton(enabled) {
    const button = this.elements.toggleButton;
    button.disabled = false;
    
    if (enabled) {
      button.textContent = 'Disable Proxy';
      button.classList.add('disabled');
    } else {
      button.textContent = 'Enable Proxy';
      button.classList.remove('disabled');
    }
  }
  
  showMessage(message, type) {
    const messageDiv = document.createElement('div');
    messageDiv.className = type === 'error' ? 'error-message' : 'success-message';
    messageDiv.textContent = message;
    
    this.elements.messageContainer.appendChild(messageDiv);
    
    // Remove message after 5 seconds
    setTimeout(() => {
      if (messageDiv.parentNode) {
        messageDiv.parentNode.removeChild(messageDiv);
      }
    }, 5000);
  }
  
  sendMessage(message) {
    return new Promise((resolve, reject) => {
      browser.runtime.sendMessage(message)
        .then(response => {
          if (response) {
            resolve(response);
          } else {
            reject(new Error('No response from background script'));
          }
        })
        .catch(reject);
    });
  }
}

// Initialize popup when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  new PopupManager();
});
