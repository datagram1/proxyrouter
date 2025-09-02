package admin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"proxyrouter/internal/config"
	"proxyrouter/internal/db"
	"proxyrouter/internal/refresh"

	"log/slog"
)

// Handlers provides HTTP handlers for the admin interface
type Handlers struct {
	config      *config.Config
	database    *db.Database
	authManager *AuthManager
	middleware  *Middleware
	refresher   *refresh.Refresher
	templates   *template.Template
}

// NewHandlers creates a new handlers instance
func NewHandlers(cfg *config.Config, database *db.Database, authManager *AuthManager, middleware *Middleware, refresher *refresh.Refresher) *Handlers {
	h := &Handlers{
		config:      cfg,
		database:    database,
		authManager: authManager,
		middleware:  middleware,
		refresher:   refresher,
	}

	// Load templates
	h.loadTemplates()

	return h
}

// loadTemplates loads HTML templates
func (h *Handlers) loadTemplates() {
	// For now, we'll use simple inline templates
	// In production, you'd load these from files
	h.templates = template.Must(template.New("admin").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}} - ProxyRouter Admin</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #f5f5f5; padding: 20px; margin-bottom: 20px; }
        .nav { background: #333; color: white; padding: 10px; }
        .nav a { color: white; text-decoration: none; margin-right: 20px; }
        .content { padding: 20px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; }
        .form-group input, .form-group textarea { width: 100%%; padding: 8px; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; cursor: pointer; }
        .btn:hover { background: #005a87; }
        .alert { padding: 10px; margin: 10px 0; border-radius: 4px; }
        .alert-success { background: #d4edda; color: #155724; }
        .alert-error { background: #f8d7da; color: #721c24; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .stat-card { background: #f8f9fa; padding: 20px; border-radius: 4px; text-align: center; }
        .stat-number { font-size: 2em; font-weight: bold; color: #007cba; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>ProxyRouter Admin</h1>
        </div>
        <div class="nav">
            <a href="/admin/">Dashboard</a>
            <a href="/admin/settings">Settings</a>
            <a href="/admin/upload">Upload Proxies</a>
            <a href="/admin/users">Users</a>
            <form method="post" action="/admin/logout" style="display: inline;">
                <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
                <button type="submit" class="btn" style="background: #dc3545;">Logout</button>
            </form>
        </div>
        <div class="content">
            {{template "content" .}}
        </div>
    </div>
</body>
</html>
`))
}

// ShowLogin displays the login form
func (h *Handlers) ShowLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Login - ProxyRouter Admin</title>
    <style>
        body { font-family: Arial, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5; }
        .login-form { background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); width: 300px; }
        .form-group { margin-bottom: 20px; }
        .form-group label { display: block; margin-bottom: 5px; }
        .form-group input { width: 100%%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        .btn { width: 100%%; background: #007cba; color: white; padding: 12px; border: none; border-radius: 4px; cursor: pointer; }
        .btn:hover { background: #005a87; }
        .error { color: #dc3545; margin-top: 10px; text-align: center; }
    </style>
</head>
<body>
    <div class="login-form">
        <h2>ProxyRouter Admin Login</h2>
        <form method="post" action="/admin/login" id="loginForm">
            <input type="hidden" name="csrf_token" id="csrfToken">
            <div class="form-group">
                <label>Username:</label>
                <input type="text" name="username" required>
            </div>
            <div class="form-group">
                <label>Password:</label>
                <input type="password" name="password" required>
            </div>
            <button type="submit" class="btn">Login</button>
            <div id="error" class="error" style="display: none;"></div>
        </form>
    </div>

    <script>
        // CSRF token management
        let csrfToken = '';
        let refreshTimer = null;

        // Function to fetch a new CSRF token
        async function refreshCSRFToken() {
            try {
                const response = await fetch('/admin/csrf-login');
                if (response.ok) {
                    const data = await response.json();
                    csrfToken = data.csrf_token;
                    document.getElementById('csrfToken').value = csrfToken;
                    console.log('CSRF token refreshed');
                } else {
                    console.error('Failed to refresh CSRF token');
                }
            } catch (error) {
                console.error('Error refreshing CSRF token:', error);
            }
        }

        // Function to refresh token periodically (every 25 minutes)
        function startTokenRefresh() {
            // Refresh immediately
            refreshCSRFToken();
            
            // Then refresh every 25 minutes (tokens are valid for 30 minutes)
            refreshTimer = setInterval(refreshCSRFToken, 25 * 60 * 1000);
        }

        // Function to stop token refresh
        function stopTokenRefresh() {
            if (refreshTimer) {
                clearInterval(refreshTimer);
                refreshTimer = null;
            }
        }

        // Handle form submission
        document.getElementById('loginForm').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            // Ensure we have a valid CSRF token
            if (!csrfToken) {
                await refreshCSRFToken();
            }
            
            // Submit the form
            this.submit();
        });

        // Start token refresh when page loads
        startTokenRefresh();

        // Clean up when page unloads
        window.addEventListener('beforeunload', stopTokenRefresh);
    </script>
</body>
</html>
`)
}

// DoLogin handles login form submission
func (h *Handlers) DoLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Authenticate user
	valid, forceChange, err := h.authManager.AuthenticateUser(r.Context(), username, password)
	if err != nil {
		slog.Error("Authentication error", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !valid {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create session
	sessionID, err := h.authManager.CreateSession(username, forceChange)
	if err != nil {
		slog.Error("Failed to create session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.config.Admin.TLS.Enabled,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	// Log audit event
	h.authManager.LogAudit(r.Context(), username, "login", "successful login", h.middleware.getClientIP(r))

	// Always redirect to dashboard
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

// Dashboard displays the admin dashboard
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get dashboard data
	stats, err := h.getDashboardStats(r.Context())
	if err != nil {
		slog.Error("Failed to get dashboard stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Check if user is using default password
	usingDefaultPassword, err := h.isUsingDefaultPassword(r.Context(), session.Username)
	if err != nil {
		slog.Error("Failed to check default password", "error", err)
		// Continue without the check
		usingDefaultPassword = false
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Dashboard - ProxyRouter Admin</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #f5f5f5; padding: 20px; margin-bottom: 20px; }
        .nav { background: #333; color: white; padding: 10px; }
        .nav a { color: white; text-decoration: none; margin-right: 20px; }
        .content { padding: 20px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .stat-card { background: #f8f9fa; padding: 20px; border-radius: 4px; text-align: center; }
        .stat-number { font-size: 2em; font-weight: bold; color: #007cba; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; cursor: pointer; margin: 5px; }
        .btn:hover { background: #005a87; }
        .alert { padding: 15px; margin: 20px 0; border-radius: 4px; border-left: 4px solid; }
        .alert-warning { background: #fff3cd; color: #856404; border-left-color: #ffc107; }
        .alert-warning .btn { background: #856404; }
        .alert-warning .btn:hover { background: #6d5204; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>ProxyRouter Admin Dashboard</h1>
            <p>Welcome, %s!</p>
        </div>
        <div class="nav">
            <a href="/admin/">Dashboard</a>
            <a href="/admin/settings">Settings</a>
            <a href="/admin/upload">Upload Proxies</a>
            <a href="/admin/users">Users</a>
            %s
            <form method="post" action="/admin/logout" style="display: inline;">
                <button type="submit" class="btn" style="background: #dc3545;">Logout</button>
            </form>
        </div>
        <div class="content">
            %s
            <h2>System Status</h2>
            <div class="stats">
                <div class="stat-card">
                    <div class="stat-number">%d</div>
                    <div>Total Proxies</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">%d</div>
                    <div>Alive Proxies</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">%d</div>
                    <div>Routes</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">%d</div>
                    <div>ACL Entries</div>
                </div>
            </div>
            
            <h2>Actions</h2>
            <form method="post" action="/admin/refresh" style="display: inline;">
                <button type="submit" class="btn">Run Manual Refresh</button>
            </form>
            <form method="post" action="/admin/health-check" style="display: inline;">
                <button type="submit" class="btn">Run Health Check</button>
            </form>
        </div>
    </div>
</body>
</html>
`, session.Username, h.getChangePasswordButton(usingDefaultPassword), h.getDefaultPasswordWarning(usingDefaultPassword), stats.TotalProxies, stats.AliveProxies, stats.Routes, stats.ACLEntries)
}

// getDashboardStats retrieves dashboard statistics
func (h *Handlers) getDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Get proxy counts
	err := h.database.GetDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM proxies").Scan(&stats.TotalProxies)
	if err != nil {
		return nil, fmt.Errorf("failed to get total proxies: %w", err)
	}

	err = h.database.GetDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM proxies WHERE alive = 1").Scan(&stats.AliveProxies)
	if err != nil {
		return nil, fmt.Errorf("failed to get alive proxies: %w", err)
	}

	// Get route count
	err = h.database.GetDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM routes").Scan(&stats.Routes)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	// Get ACL count
	err = h.database.GetDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM acl_subnets").Scan(&stats.ACLEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL entries: %w", err)
	}

	return stats, nil
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalProxies int `json:"total_proxies"`
	AliveProxies int `json:"alive_proxies"`
	Routes       int `json:"routes"`
	ACLEntries   int `json:"acl_entries"`
}

// HealthSummary provides detailed health information
func (h *Handlers) HealthSummary(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"uptime":    "0s", // TODO: Calculate actual uptime
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// GetSettings displays the settings form
func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement settings form
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h1>Settings</h1><p>Settings form will be implemented here.</p>")
}

// PostSettings handles settings form submission
func (h *Handlers) PostSettings(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement settings update
	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}

// UploadForm displays the proxy upload form
func (h *Handlers) UploadForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate CSRF token
	csrfToken := h.middleware.generateCSRFToken(session.Username)

	// Get success message parameters
	imported := r.URL.Query().Get("imported")
	updated := r.URL.Query().Get("updated")
	skipped := r.URL.Query().Get("skipped")

	var successMessage string
	if imported != "" {
		successMessage = fmt.Sprintf(`
			<div class="alert alert-success">
				<strong>Upload successful!</strong><br>
				Imported: %s | Updated: %s | Skipped: %s
			</div>
		`, imported, updated, skipped)
	}
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Upload Proxies - ProxyRouter Admin</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 800px; margin: 0 auto; }
        .header { background: #f5f5f5; padding: 20px; margin-bottom: 20px; }
        .nav { background: #333; color: white; padding: 10px; }
        .nav a { color: white; text-decoration: none; margin-right: 20px; }
        .content { padding: 20px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; }
        .form-group input, .form-group textarea { width: 100%%; padding: 8px; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; cursor: pointer; }
        .btn:hover { background: #005a87; }
        .btn:disabled { background: #6c757d; cursor: not-allowed; }
        .input-method { margin-bottom: 10px; padding: 10px; border-radius: 4px; }
        .input-method.file { background: #e7f3ff; border-left: 4px solid #007cba; }
        .input-method.text { background: #fff3cd; border-left: 4px solid #ffc107; }
        .input-method.none { background: #f8d7da; border-left: 4px solid #dc3545; }
        .alert { padding: 15px; margin: 20px 0; border-radius: 4px; }
        .alert-success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Upload Proxies</h1>
        </div>
        <div class="nav">
            <a href="/admin/">Dashboard</a>
            <a href="/admin/settings">Settings</a>
            <a href="/admin/upload">Upload Proxies</a>
            <a href="/admin/users">Users</a>
            <form method="post" action="/admin/logout" style="display: inline;">
                <button type="submit" class="btn" style="background: #dc3545;">Logout</button>
            </form>
        </div>
        <div class="content">
            %s
            <h2>Upload Proxy List</h2>
            <div id="input-method-indicator" class="input-method none">
                <strong>Please provide either a file or paste proxies in the text area below.</strong>
            </div>
            <form method="post" action="/admin/upload" enctype="multipart/form-data" id="upload-form">
                <input type="hidden" name="csrf_token" value="%s">
                <div class="form-group">
                    <label>File (.txt or .csv):</label>
                    <input type="file" name="file" accept=".txt,.csv" id="file-input">
                </div>
                <div class="form-group">
                    <label>Or paste proxies (one per line, format: host:port or scheme://host:port):</label>
                    <textarea name="proxies" rows="10" placeholder="127.0.0.1:8080&#10;http://proxy.example.com:3128&#10;socks5://socks.example.com:1080" id="proxies-textarea"></textarea>
                </div>
                <button type="submit" class="btn" id="submit-btn" disabled>Upload Proxies</button>
            </form>
        </div>
    </div>
    
    <script>
        const fileInput = document.getElementById('file-input');
        const textarea = document.getElementById('proxies-textarea');
        const submitBtn = document.getElementById('submit-btn');
        const indicator = document.getElementById('input-method-indicator');
        
        function updateFormState() {
            const hasFile = fileInput.files && fileInput.files.length > 0;
            const hasText = textarea.value.trim() !== '';
            
            // Check if there's a success message on the page
            const successMessage = document.querySelector('.alert-success');
            if (successMessage) {
                // If there's a success message, hide the indicator and disable the submit button
                indicator.style.display = 'none';
                submitBtn.disabled = true;
                return;
            }
            
            // Show the indicator if there's no success message
            indicator.style.display = 'block';
            
            if (hasFile && hasText) {
                indicator.className = 'input-method file';
                indicator.innerHTML = '<strong>File upload will be used.</strong> Text area content will be ignored.';
                submitBtn.disabled = false;
            } else if (hasFile) {
                indicator.className = 'input-method file';
                indicator.innerHTML = '<strong>File upload will be used.</strong>';
                submitBtn.disabled = false;
            } else if (hasText) {
                indicator.className = 'input-method text';
                indicator.innerHTML = '<strong>Text area content will be used.</strong>';
                submitBtn.disabled = false;
            } else {
                indicator.className = 'input-method none';
                indicator.innerHTML = '<strong>Please provide either a file or paste proxies in the text area below.</strong>';
                submitBtn.disabled = true;
            }
        }
        
        fileInput.addEventListener('change', updateFormState);
        textarea.addEventListener('input', updateFormState);
        
        // Initialize state
        updateFormState();
    </script>
</body>
</html>
`, successMessage, csrfToken)
}

// UploadProxies handles proxy upload
func (h *Handlers) UploadProxies(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	var proxies []string

	// Handle file upload
	if file, header, err := r.FormFile("file"); err == nil {
		defer file.Close()

		// Parse based on file extension
		if strings.HasSuffix(header.Filename, ".csv") {
			proxies, err = h.parseCSVFile(file)
		} else {
			proxies, err = h.parseTextFile(file)
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse file: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// Handle text input
		text := r.FormValue("proxies")
		if text == "" {
			http.Error(w, "No file or text provided", http.StatusBadRequest)
			return
		}

		proxies = h.parseTextInput(text)
	}

	// Import proxies
	imported, updated, skipped, err := h.importProxies(r.Context(), proxies)
	if err != nil {
		slog.Error("Failed to import proxies", "error", err)
		http.Error(w, "Failed to import proxies", http.StatusInternalServerError)
		return
	}

	// Redirect with success message
	http.Redirect(w, r, fmt.Sprintf("/admin/upload?imported=%d&updated=%d&skipped=%d", imported, updated, skipped), http.StatusSeeOther)
}

// parseCSVFile parses a CSV file containing proxies
func (h *Handlers) parseCSVFile(file io.Reader) ([]string, error) {
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var proxies []string
	for _, record := range records {
		if len(record) >= 2 {
			host := record[0]
			port := record[1]
			scheme := "http"
			if len(record) >= 3 && record[2] != "" {
				scheme = record[2]
			}
			proxies = append(proxies, fmt.Sprintf("%s://%s:%s", scheme, host, port))
		}
	}

	return proxies, nil
}

// parseTextFile parses a text file containing proxies
func (h *Handlers) parseTextFile(file io.Reader) ([]string, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return h.parseTextInput(string(content)), nil
}

// parseTextInput parses text input containing proxies
func (h *Handlers) parseTextInput(text string) []string {
	lines := strings.Split(text, "\n")
	var proxies []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// If line doesn't have a scheme, assume http
		if !strings.Contains(line, "://") {
			line = "http://" + line
		}

		proxies = append(proxies, line)
	}

	return proxies
}

// importProxies imports proxies into the database
func (h *Handlers) importProxies(ctx context.Context, proxies []string) (imported, updated, skipped int, err error) {
	// TODO: Implement proxy import logic
	// This would use the refresh package to import proxies
	return len(proxies), 0, 0, nil
}

// ListUsers displays the user management page
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement user list
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h1>Users</h1><p>User management will be implemented here.</p>")
}

// ChangePassword handles password change
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Get session from context
		session, ok := r.Context().Value("session").(*Session)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Generate CSRF token
		csrfToken := h.middleware.generateCSRFToken(session.Username)

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Change Password - ProxyRouter Admin</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 600px; margin: 0 auto; }
        .header { background: #f5f5f5; padding: 20px; margin-bottom: 20px; }
        .content { padding: 20px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; }
        .form-group input { width: 100%%; padding: 8px; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; cursor: pointer; }
        .btn:hover { background: #005a87; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Change Password</h1>
        </div>
        <div class="content">
            <form method="post" action="/admin/users/change-password">
                <input type="hidden" name="csrf_token" value="%s">
                <div class="form-group">
                    <label>Current Password:</label>
                    <input type="password" name="current_password" required>
                </div>
                <div class="form-group">
                    <label>New Password:</label>
                    <input type="password" name="new_password" required>
                </div>
                <div class="form-group">
                    <label>Confirm New Password:</label>
                    <input type="password" name="confirm_password" required>
                </div>
                <button type="submit" class="btn">Change Password</button>
            </form>
        </div>
    </div>
</body>
</html>
`, csrfToken)
		return
	}

	// Handle POST request
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate passwords
	if newPassword != confirmPassword {
		http.Error(w, "New passwords do not match", http.StatusBadRequest)
		return
	}

	if len(newPassword) < 8 {
		http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Change password
	err := h.authManager.ChangePassword(r.Context(), session.Username, currentPassword, newPassword)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to change password: %v", err), http.StatusBadRequest)
		return
	}

	// Log audit event
	h.authManager.LogAudit(r.Context(), session.Username, "change_password", "password changed", h.middleware.getClientIP(r))

	// Redirect to dashboard
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

// CreateUser handles user creation
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement user creation
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// RefreshCSRFToken handles CSRF token refresh requests
func (h *Handlers) RefreshCSRFToken(w http.ResponseWriter, r *http.Request) {
	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate new CSRF token
	newToken := h.middleware.refreshCSRFToken(session.Username)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"csrf_token": "%s"}`, newToken)
}

// GetLoginCSRFToken generates a CSRF token for the login page
func (h *Handlers) GetLoginCSRFToken(w http.ResponseWriter, r *http.Request) {
	// Generate a temporary CSRF token for login (valid for 1 hour)
	// We'll use a special username "login" for this purpose
	expiry := time.Now().Add(1 * time.Hour)
	token := h.middleware.generateCSRFTokenWithExpiry("login", expiry)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"csrf_token": "%s"}`, token)
}

// DoLogout handles logout
func (h *Handlers) DoLogout(w http.ResponseWriter, r *http.Request) {
	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if ok {
		// Log audit event
		h.authManager.LogAudit(r.Context(), session.Username, "logout", "user logged out", h.middleware.getClientIP(r))
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Redirect to login
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// isUsingDefaultPassword checks if the user is using the default password
func (h *Handlers) isUsingDefaultPassword(ctx context.Context, username string) (bool, error) {
	// Try to authenticate with the default password
	valid, _, err := h.authManager.AuthenticateUser(ctx, username, "admin")
	if err != nil {
		return false, fmt.Errorf("failed to check default password: %w", err)
	}
	return valid, nil
}

// getDefaultPasswordWarning returns the HTML for the default password warning
func (h *Handlers) getDefaultPasswordWarning(usingDefaultPassword bool) string {
	if !usingDefaultPassword {
		return ""
	}

	return `
	<div class="alert alert-warning">
		<strong>Security Warning:</strong> You are currently using the default password (admin/admin). 
		For security reasons, we strongly recommend changing your password.
	</div>
	`
}

// getChangePasswordButton returns the HTML for the change password button in the nav bar
func (h *Handlers) getChangePasswordButton(usingDefaultPassword bool) string {
	if !usingDefaultPassword {
		return ""
	}

	return `<a href="/admin/users/change-password" class="btn" style="background: #856404; margin-right: 10px;">Change Password</a>`
}

// RunRefresh handles manual refresh requests
func (h *Handlers) RunRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Run refresh
	if err := h.refresher.RefreshAll(r.Context()); err != nil {
		slog.Error("Failed to run refresh", "error", err)
		http.Error(w, fmt.Sprintf("Failed to run refresh: %v", err), http.StatusInternalServerError)
		return
	}

	// Log audit event
	h.authManager.LogAudit(r.Context(), session.Username, "refresh", "manual refresh triggered", h.middleware.getClientIP(r))

	// Redirect back to dashboard with success message
	http.Redirect(w, r, "/admin/?refresh=success", http.StatusSeeOther)
}

// RunHealthCheck handles manual health check requests
func (h *Handlers) RunHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session from context
	session, ok := r.Context().Value("session").(*Session)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Run health check
	if err := h.refresher.HealthCheck(r.Context()); err != nil {
		slog.Error("Failed to run health check", "error", err)
		http.Error(w, fmt.Sprintf("Failed to run health check: %v", err), http.StatusInternalServerError)
		return
	}

	// Log audit event
	h.authManager.LogAudit(r.Context(), session.Username, "health_check", "manual health check triggered", h.middleware.getClientIP(r))

	// Redirect back to dashboard with success message
	http.Redirect(w, r, "/admin/?health_check=success", http.StatusSeeOther)
}
