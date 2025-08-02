<?php
// Extract plugin zip file using PHP
$zip = new ZipArchive;
if ($zip->open('/tmp/all-in-one-wp-migration.zip') === TRUE) {
    $zip->extractTo('/var/www/html/wp-content/plugins/');
    $zip->close();
    echo "Plugin extracted successfully\n";
    unlink('/tmp/all-in-one-wp-migration.zip');
} else {
    echo "Failed to open zip file\n";
    exit(1);
}
