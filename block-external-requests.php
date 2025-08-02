<?php
// Block external HTTP requests that might be causing delays

// Disable WordPress update checks
add_filter('pre_site_transient_update_core', '__return_null');
add_filter('pre_site_transient_update_plugins', '__return_null');
add_filter('pre_site_transient_update_themes', '__return_null');

// Block most external HTTP requests in admin
add_filter('pre_http_request', function($preempt, $args, $url) {
    if (is_admin()) {
        // Allow local requests
        if (strpos($url, home_url()) === 0) {
            return false;
        }
        
        // Block external requests but log them
        error_log('Blocked external request to: ' . $url);
        return new WP_Error('http_request_blocked', 'External HTTP request blocked for performance');
    }
    return false;
}, 10, 3);

// Disable plugin/theme update notifications
remove_action('load-update-core.php', 'wp_update_plugins');
remove_action('load-update-core.php', 'wp_update_themes');

// Disable WordPress news widget
add_action('wp_dashboard_setup', function() {
    remove_meta_box('dashboard_primary', 'dashboard', 'side');
});
