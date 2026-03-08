#!/bin/bash
# Run as cron job: 0 3 * * * /path/to/certbot-renew.sh
certbot renew --quiet
docker compose -f "$(dirname "$0")/docker-compose.yml" restart nginx
