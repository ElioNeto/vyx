package gateway

// SecurityHeaders returns the standard set of security response headers
// that the gateway adds to every outbound response.
//
// The Content-Security-Policy is intentionally permissive with styles and
// scripts because vyx workers render HTML server-side with inline <style>
// and <script> blocks. Workers may also integrate external resources
// (e.g. Google Fonts, analytics) via the CDN allowlist.
func SecurityHeaders() map[string]string {
	return map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; script-src 'self' 'unsafe-inline'",
		"Permissions-Policy":       "geolocation=(), camera=(), microphone=(), payment=(), usb=()",
	}
}
