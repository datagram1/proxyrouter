# ProxyRouter Firefox Extension

A Firefox browser extension that routes all Firefox traffic through your ProxyRouter server.

## Features

- **Easy Proxy Toggle**: One-click enable/disable of proxy routing
- **Real-time Status**: Shows connection status and proxy statistics
- **Health Monitoring**: View proxy health and trigger health checks
- **Configurable Settings**: Customize proxy host, ports, and routing modes
- **API Integration**: Direct integration with ProxyRouter's REST API

## Installation

### Prerequisites

1. **ProxyRouter Server**: Make sure your ProxyRouter server is running
   - HTTP Proxy: `localhost:8080` (default)
   - API Server: `localhost:8081` (default)

2. **Firefox Browser**: Firefox 57+ (Quantum)

### Installation Steps

1. **Download the Extension**
   ```bash
   # Clone or download the firefox-extension folder
   cd proxyrouter/firefox-extension
   ```

2. **Load Extension in Firefox**
   - Open Firefox
   - Navigate to `about:debugging`
   - Click "This Firefox" tab
   - Click "Load Temporary Add-on"
   - Select the `manifest.json` file from the firefox-extension folder

3. **Configure Settings**
   - Click the ProxyRouter extension icon in the toolbar
   - Click "Settings" to configure your ProxyRouter server details
   - Set your proxy host, port, and API details
   - Click "Test Connection" to verify connectivity

## Usage

### Basic Usage

1. **Enable Proxy**: Click the extension icon and click "Enable Proxy"
2. **Browse Normally**: All Firefox traffic will now route through ProxyRouter
3. **Monitor Status**: The extension shows real-time connection status and proxy statistics

### Advanced Features

- **Health Checks**: Click "Health Check" to test all proxies
- **Statistics**: View total and alive proxy counts
- **Routing Modes**: Configure different routing modes (LOCAL, GENERAL, TOR, UPSTREAM)

### Configuration Options

| Setting | Description | Default |
|---------|-------------|---------|
| Proxy Host | ProxyRouter server hostname | localhost |
| Proxy Port | HTTP proxy port | 8080 |
| API Host | API server hostname | localhost |
| API Port | API server port | 8081 |
| Routing Mode | Default routing mode | GENERAL |
| Auto Switch | Auto-switch based on website | false |

## Troubleshooting

### Common Issues

1. **"Cannot connect to ProxyRouter server"**
   - Ensure ProxyRouter is running: `./proxyrouter`
   - Check firewall settings
   - Verify host and port settings

2. **"API connection failed"**
   - Verify API port (default: 8081)
   - Check ProxyRouter logs for API errors
   - Ensure ACL allows your IP address

3. **Extension not working**
   - Reload the extension in `about:debugging`
   - Check Firefox console for errors
   - Verify manifest.json is valid

### Debug Mode

1. Open Firefox Developer Tools (F12)
2. Go to Console tab
3. Look for "ProxyRouter:" messages
4. Check for any error messages

## Development

### File Structure

```
firefox-extension/
├── manifest.json      # Extension manifest
├── background.js       # Background script (proxy logic)
├── popup.html         # Popup interface
├── popup.js           # Popup logic
├── options.html       # Settings page
├── options.js         # Settings logic
├── icons/             # Extension icons
└── README.md          # This file
```

### Building for Production

To create a production-ready extension:

1. **Create Icons**: Add proper icons in the `icons/` folder
2. **Update Version**: Update version in `manifest.json`
3. **Test Thoroughly**: Test all features and edge cases
4. **Package**: Zip the folder for distribution

### API Integration

The extension integrates with ProxyRouter's REST API:

- `GET /api/v1/healthz` - Server health check
- `GET /api/v1/proxies?alive=1` - Get proxy statistics
- `POST /api/v1/proxies/health-check` - Trigger health check

## Security Notes

- The extension only connects to localhost by default
- No data is sent to external servers
- All configuration is stored locally in Firefox
- Proxy credentials are not stored by the extension

## Support

For issues with the extension:
1. Check the troubleshooting section above
2. Review Firefox console logs
3. Verify ProxyRouter server status
4. Check extension permissions in Firefox

For ProxyRouter server issues, refer to the main ProxyRouter documentation.
