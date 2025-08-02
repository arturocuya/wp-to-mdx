<?php
// Block external HTTP requests that might be causing delays

// Disable WordPress update checks and plugin/theme browsing APIs
add_filter('pre_site_transient_update_core', '__return_null');
add_filter('pre_site_transient_update_plugins', '__return_null');
add_filter('pre_site_transient_update_themes', '__return_null');

// Block plugin API calls that slow down admin
add_filter('plugins_api', function($result, $action, $args) {
    if ($action === 'query_plugins') {
        // Return empty result for plugin search to avoid delays
        return (object) array(
            'info' => array('results' => 0),
            'plugins' => array(),
        );
    }
    if ($action === 'plugin_information') {
        return new WP_Error('plugins_api_failed', 'Plugin information unavailable for performance optimization');
    }
    return $result;
}, 10, 3);

// Block theme API calls
add_filter('themes_api', function($result, $action, $args) {
    // Return empty result for theme search
    return (object) array(
        'info' => array('results' => 0),
        'themes' => array(),
    );
}, 10, 3);

// Completely block external HTTP requests in admin
add_filter('pre_http_request', function($preempt, $args, $url) {
    if (is_admin()) {
        // Allow local requests only
        if (strpos($url, home_url()) === 0) {
            return false;
        }
        
        // Block all external requests
        error_log('Blocked external request to: ' . $url);
        return new WP_Error('http_request_blocked', 'External HTTP request blocked for performance');
    }
    return false;
}, 10, 3);

// Disable plugin/theme update notifications
remove_action('load-update-core.php', 'wp_update_plugins');
remove_action('load-update-core.php', 'wp_update_themes');

// Disable WordPress news and plugin directory widgets
add_action('wp_dashboard_setup', function() {
    remove_meta_box('dashboard_primary', 'dashboard', 'side');
});

// Keep plugin installation menu but show message
add_action('admin_notices', function() {
    $screen = get_current_screen();
    if ($screen && $screen->id === 'plugin-install') {
        echo '<div class="notice notice-info"><p><strong>Plugin installation from repository is disabled for performance optimization.</strong> To install plugins, upload them manually via the "Upload Plugin" tab.</p></div>';
    }
});
