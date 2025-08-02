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

// Increase upload limits for All in One WP Migration
add_filter('wp_max_upload_size', function($size) {
    return 2 * 1024 * 1024 * 1024; // 2GB
});

add_filter('upload_size_limit', function($size) {
    return 2 * 1024 * 1024 * 1024; // 2GB
});

// Override WordPress upload limits
add_filter('pre_option_upload_max_filesize', function() {
    return '2048M';
});

add_filter('pre_option_post_max_size', function() {
    return '2048M';
});
