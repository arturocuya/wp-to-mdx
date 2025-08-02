<?php
// Temporary debugging to identify slow queries and processes

// Enable query debugging
define('SAVEQUERIES', true);

// Log slow queries
add_action('shutdown', function() {
    global $wpdb;
    
    if (defined('SAVEQUERIES') && SAVEQUERIES && is_admin()) {
        $slow_queries = array();
        foreach ($wpdb->queries as $query) {
            if ($query[1] > 0.1) { // queries taking more than 100ms
                $slow_queries[] = array(
                    'query' => $query[0],
                    'time' => $query[1],
                    'stack' => $query[2]
                );
            }
        }
        
        if (!empty($slow_queries)) {
            error_log('SLOW QUERIES DETECTED:');
            foreach ($slow_queries as $sq) {
                error_log(sprintf('Time: %fs - Query: %s', $sq['time'], substr($sq['query'], 0, 200)));
            }
        }
        
        $total_time = array_sum(array_column($wpdb->queries, 1));
        error_log(sprintf('Total DB queries: %d, Total time: %fs', count($wpdb->queries), $total_time));
    }
});

// Track page load times
add_action('init', function() {
    if (is_admin()) {
        define('WP_START_TIME', microtime(true));
    }
});

add_action('shutdown', function() {
    if (defined('WP_START_TIME') && is_admin()) {
        $load_time = microtime(true) - WP_START_TIME;
        error_log(sprintf('Admin page load time: %fs for %s', $load_time, $_SERVER['REQUEST_URI']));
    }
});
