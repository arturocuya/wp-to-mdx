# Dockerfile
FROM wordpress:5.6.1

# Install OPcache for performance
RUN docker-php-ext-install opcache

# Copy PHP settings
COPY opcache.ini /usr/local/etc/php/conf.d/opcache.ini
COPY php.ini /usr/local/etc/php/conf.d/custom.ini

# Disable Xdebug entirely (remove or override its .ini)
COPY xdebug.ini /usr/local/etc/php/conf.d/xdebug-disable.ini

# Add PHP-FPM pool overrides
COPY www.conf /usr/local/etc/php-fpm.d/www.conf

# Create mu-plugins directory and add admin performance optimizations
RUN mkdir -p /var/www/html/wp-content/mu-plugins
COPY admin-optimizations.php /var/www/html/wp-content/mu-plugins/admin-optimizations.php
COPY debug-queries.php /var/www/html/wp-content/mu-plugins/debug-queries.php
COPY block-external-requests.php /var/www/html/wp-content/mu-plugins/block-external-requests.php

# WP code lives in image; uploads persisted via compose volume