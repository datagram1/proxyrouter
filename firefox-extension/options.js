// ProxyRouter Firefox Extension Options Script
// Handles settings form and configuration management

class OptionsManager {
  constructor() {
    this.defaultConfig = {
      enabled: false,
      proxyHost: 'localhost',
      proxyPort: 8080,
      apiHost: 'localhost',
      apiPort: 8081,
      routingMode: 'GENERAL',
      autoSwitch: false
    };
    
    this.elements = {
      form: document.getElementById('settings-form'),
      statusMessage: document.getElementById('status-message'),
      saveButton: document.getElementById('save-button'),
      resetButton: document.getElementById('reset-button'),
      testConnectionButton: document.getElementById('test-connection')
    };
    
    this.init();
  }
  
  async init() {
    this.setupEventListeners();
    await this.loadSettings();
  }
  
  setupEventListeners() {
    this.elements.form.addEventListener('submit', this.handleSave.bind(this));
    this.elements.resetButton.addEventListener('click', this.handleReset.bind(this));
    this.elements.testConnectionButton.addEventListener('click', this.handleTestConnection.bind(this));
  }
  
  async loadSettings() {
    try {
      const result = await browser.storage.local.get('proxyRouterConfig');
      const config = { ...this.defaultConfig, ...result.proxyRouterConfig };
      
      // Populate form fields
      document.getElementById('proxyHost').value = config.proxyHost;
      document.getElementById('proxyPort').value = config.proxyPort;
      document.getElementById('apiHost').value = config.apiHost;
      document.getElementById('apiPort').value = config.apiPort;
      document.getElementById('routingMode').value = config.routingMode;
      document.getElementById('autoSwitch').checked = config.autoSwitch;
      
    } catch (error) {
      console.error('Failed to load settings:', error);
      this.showMessage('Failed to load settings', 'error');
    }
  }
  
  async handleSave(event) {
    event.preventDefault();
    
    try {
      this.elements.saveButton.disabled = true;
      this.elements.saveButton.textContent = 'Saving...';
      
      const config = {
        enabled: false, // Will be set by background script
        proxyHost: document.getElementById('proxyHost').value.trim(),
        proxyPort: parseInt(document.getElementById('proxyPort').value),
        apiHost: document.getElementById('apiHost').value.trim(),
        apiPort: parseInt(document.getElementById('apiPort').value),
        routingMode: document.getElementById('routingMode').value,
        autoSwitch: document.getElementById('autoSwitch').checked
      };
      
      // Validate required fields
      if (!config.proxyHost || !config.apiHost) {
        throw new Error('Host fields are required');
      }
      
      if (config.proxyPort < 1 || config.proxyPort > 65535) {
        throw new Error('Proxy port must be between 1 and 65535');
      }
      
      if (config.apiPort < 1 || config.apiPort > 65535) {
        throw new Error('API port must be between 1 and 65535');
      }
      
      // Save configuration
      await this.sendMessage({ action: 'updateConfig', config });
      
      this.showMessage('Settings saved successfully', 'success');
      
    } catch (error) {
      console.error('Save failed:', error);
      this.showMessage(error.message || 'Failed to save settings', 'error');
    } finally {
      this.elements.saveButton.disabled = false;
      this.elements.saveButton.textContent = 'Save Settings';
    }
  }
  
  async handleReset() {
    try {
      this.elements.resetButton.disabled = true;
      this.elements.resetButton.textContent = 'Resetting...';
      
      // Reset form to defaults
      document.getElementById('proxyHost').value = this.defaultConfig.proxyHost;
      document.getElementById('proxyPort').value = this.defaultConfig.proxyPort;
      document.getElementById('apiHost').value = this.defaultConfig.apiHost;
      document.getElementById('apiPort').value = this.defaultConfig.apiPort;
      document.getElementById('routingMode').value = this.defaultConfig.routingMode;
      document.getElementById('autoSwitch').checked = this.defaultConfig.autoSwitch;
      
      // Save default configuration
      await this.sendMessage({ action: 'updateConfig', config: this.defaultConfig });
      
      this.showMessage('Settings reset to defaults', 'success');
      
    } catch (error) {
      console.error('Reset failed:', error);
      this.showMessage('Failed to reset settings', 'error');
    } finally {
      this.elements.resetButton.disabled = false;
      this.elements.resetButton.textContent = 'Reset to Defaults';
    }
  }
  
  async handleTestConnection() {
    try {
      this.elements.testConnectionButton.disabled = true;
      this.elements.testConnectionButton.textContent = 'Testing...';
      
      // Get current form values
      const config = {
        proxyHost: document.getElementById('proxyHost').value.trim(),
        proxyPort: parseInt(document.getElementById('proxyPort').value),
        apiHost: document.getElementById('apiHost').value.trim(),
        apiPort: parseInt(document.getElementById('apiPort').value)
      };
      
      // Test API connection
      const apiUrl = `http://${config.apiHost}:${config.apiPort}/api/v1/healthz`;
      
      const response = await fetch(apiUrl, {
        method: 'GET',
        timeout: 5000
      });
      
      if (response.ok) {
        const data = await response.json();
        this.showMessage(`Connection successful! Server status: ${data.status}`, 'success');
      } else {
        this.showMessage(`API connection failed: HTTP ${response.status}`, 'error');
      }
      
    } catch (error) {
      console.error('Test connection failed:', error);
      this.showMessage(`Connection failed: ${error.message}`, 'error');
    } finally {
      this.elements.testConnectionButton.disabled = false;
      this.elements.testConnectionButton.textContent = 'Test Connection';
    }
  }
  
  showMessage(message, type) {
    const messageDiv = this.elements.statusMessage;
    messageDiv.textContent = message;
    messageDiv.className = `status-message status-${type}`;
    messageDiv.style.display = 'block';
    
    // Hide message after 5 seconds
    setTimeout(() => {
      messageDiv.style.display = 'none';
    }, 5000);
  }
  
  sendMessage(message) {
    return new Promise((resolve, reject) => {
      browser.runtime.sendMessage(message)
        .then(response => {
          if (response && response.success) {
            resolve(response);
          } else {
            reject(new Error('Failed to update configuration'));
          }
        })
        .catch(reject);
    });
  }
}

// Initialize options page when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  new OptionsManager();
});
