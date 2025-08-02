<?php
// Admin performance optimizations for WordPress
// This file is automatically loaded to optimize admin performance

// Disable admin heartbeat on non-essential pages
add_action('init', function() {
    if (is_admin() && !wp_doing_ajax()) {
        wp_deregister_script('heartbeat');
    }
});

// Remove unnecessary admin widgets
add_action('wp_dashboard_setup', function() {
    global $wp_meta_boxes;
    unset($wp_meta_boxes['dashboard']['side']['core']['dashboard_quick_press']);
    unset($wp_meta_boxes['dashboard']['normal']['core']['dashboard_incoming_links']);
    unset($wp_meta_boxes['dashboard']['normal']['core']['dashboard_plugins']);
    unset($wp_meta_boxes['dashboard']['normal']['core']['dashboard_recent_comments']);
    unset($wp_meta_boxes['dashboard']['side']['core']['dashboard_recent_drafts']);
    unset($wp_meta_boxes['dashboard']['normal']['core']['dashboard_recent_comments']);
});

// Limit admin posts per page to reduce query load
add_filter('edit_posts_per_page', function() {
    return 20;
});

// Disable admin email verification
add_filter('admin_email_check_interval', '__return_false');

// Remove version query strings from static resources
add_filter('script_loader_src', function($src) {
    if (strpos($src, 'ver=')) {
        $src = remove_query_arg('ver', $src);
    }
    return $src;
});

add_filter('style_loader_src', function($src) {
    if (strpos($src, 'ver=')) {
        $src = remove_query_arg('ver', $src);
    }
    return $src;
});
