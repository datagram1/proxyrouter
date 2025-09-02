// ProxyRouter Firefox Extension Background Script
// Handles proxy configuration and API communication

class ProxyRouterExtension {
  constructor() {
    this.defaultConfig = {
      enabled: false,
      proxyHost: 'localhost',
      proxyPort: 8080,
      apiHost: 'localhost',
      apiPort: 8081,
      routingMode: 'GENERAL', // LOCAL, GENERAL, TOR, UPSTREAM
      autoSwitch: false
    };
    
    this.init();
  }
  
  async init() {
    // Load saved configuration
    const config = await this.loadConfig();
    
    // Set up proxy configuration
    if (config.enabled) {
      this.setupProxy(config);
    }
    
    // Listen for configuration changes
    browser.storage.onChanged.addListener(this.handleStorageChange.bind(this));
  }
  
  async loadConfig() {
    const result = await browser.storage.local.get('proxyRouterConfig');
    return { ...this.defaultConfig, ...result.proxyRouterConfig };
  }
  
  async saveConfig(config) {
    await browser.storage.local.set({ proxyRouterConfig: config });
  }
  
  setupProxy(config) {
    const proxyConfig = {
      proxyType: "manual",
      http: `${config.proxyHost}:${config.proxyPort}`,
      httpProxy: `${config.proxyHost}:${config.proxyPort}`,
      ssl: `${config.proxyHost}:${config.proxyPort}`,
      sslProxy: `${config.proxyHost}:${config.proxyPort}`,
      ftp: `${config.proxyHost}:${config.proxyPort}`,
      ftpProxy: `${config.proxyHost}:${config.proxyPort}`,
      noProxy: "localhost,127.0.0.1"
    };
    
    browser.proxy.settings.set({
      value: proxyConfig,
      scope: 'regular'
    });
    
    console.log('ProxyRouter: Proxy configured for', config.proxyHost + ':' + config.proxyPort);
  }
  
  clearProxy() {
    browser.proxy.settings.clear({
      scope: 'regular'
    });
    
    console.log('ProxyRouter: Proxy configuration cleared');
  }
  
  async handleStorageChange(changes, areaName) {
    if (areaName === 'local' && changes.proxyRouterConfig) {
      const newConfig = changes.proxyRouterConfig.newValue;
      const oldConfig = changes.proxyRouterConfig.oldValue;
      
      if (newConfig.enabled !== oldConfig?.enabled) {
        if (newConfig.enabled) {
          this.setupProxy(newConfig);
        } else {
          this.clearProxy();
        }
      }
    }
  }
  
  async getProxyStatus() {
    try {
      const config = await this.loadConfig();
      const apiUrl = `http://${config.apiHost}:${config.apiPort}/api/v1/healthz`;
      
      const response = await fetch(apiUrl, {
        method: 'GET',
        timeout: 5000
      });
      
      if (response.ok) {
        const data = await response.json();
        return {
          status: 'connected',
          message: 'ProxyRouter server is running',
          data: data
        };
      } else {
        return {
          status: 'error',
          message: `Server responded with ${response.status}`,
          data: null
        };
      }
    } catch (error) {
      return {
        status: 'error',
        message: 'Cannot connect to ProxyRouter server',
        error: error.message
      };
    }
  }
  
  async getProxyStats() {
    try {
      const config = await this.loadConfig();
      const apiUrl = `http://${config.apiHost}:${config.apiPort}/api/v1/proxies?alive=1`;
      
      const response = await fetch(apiUrl, {
        method: 'GET',
        timeout: 5000
      });
      
      if (response.ok) {
        const data = await response.json();
        return {
          status: 'success',
          proxies: data.proxies || [],
          totalProxies: data.proxies ? data.proxies.length : 0,
          aliveProxies: data.proxies ? data.proxies.filter(p => p.alive).length : 0
        };
      } else {
        return {
          status: 'error',
          message: `Failed to fetch proxy stats: ${response.status}`,
          proxies: []
        };
      }
    } catch (error) {
      return {
        status: 'error',
        message: 'Cannot fetch proxy statistics',
        error: error.message,
        proxies: []
      };
    }
  }
  
  async triggerHealthCheck() {
    try {
      const config = await this.loadConfig();
      const apiUrl = `http://${config.apiHost}:${config.apiPort}/api/v1/proxies/health-check`;
      
      const response = await fetch(apiUrl, {
        method: 'POST',
        timeout: 30000 // Health check can take time
      });
      
      if (response.ok) {
        const data = await response.json();
        return {
          status: 'success',
          message: 'Health check completed',
          data: data
        };
      } else {
        return {
          status: 'error',
          message: `Health check failed: ${response.status}`,
          data: null
        };
      }
    } catch (error) {
      return {
        status: 'error',
        message: 'Health check failed',
        error: error.message
      };
    }
  }
}

// Initialize the extension
const proxyRouter = new ProxyRouterExtension();

// Handle messages from popup and options pages
browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
  switch (message.action) {
    case 'getStatus':
      proxyRouter.getProxyStatus().then(sendResponse);
      return true; // Keep message channel open for async response
      
    case 'getStats':
      proxyRouter.getProxyStats().then(sendResponse);
      return true;
      
    case 'triggerHealthCheck':
      proxyRouter.triggerHealthCheck().then(sendResponse);
      return true;
      
    case 'toggleProxy':
      proxyRouter.loadConfig().then(config => {
        config.enabled = !config.enabled;
        proxyRouter.saveConfig(config);
        sendResponse({ success: true, enabled: config.enabled });
      });
      return true;
      
    case 'updateConfig':
      proxyRouter.saveConfig(message.config).then(() => {
        sendResponse({ success: true });
      });
      return true;
  }
});
