#!/bin/sh
set -e

# Chown the log directory to the app user
if [ -d /var/log/codecurfew ]; then
    chown -R app:app /var/log/codecurfew
fi

# Execute the app as the app user
exec su-exec app:app codecurfew "$@"
