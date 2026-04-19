#!/usr/bin/env bash
set -euo pipefail

# Backup Marrow SQLite database
# Usage: ./backup.sh /var/lib/marrow/data/marrow.db

DB_PATH="${1:-marrow.db}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
S3_BUCKET="${S3_BUCKET:-}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="marrow_${TIMESTAMP}.db"
DEST="$BACKUP_DIR/$BACKUP_FILE"

mkdir -p "$BACKUP_DIR"

echo "==> Backing up $DB_PATH to $DEST"
cp "$DB_PATH" "$DEST"

# Optional: upload to S3
if [ -n "$S3_BUCKET" ]; then
    if command -v aws &>/dev/null; then
        echo "==> Uploading to s3://$S3_BUCKET/$BACKUP_FILE"
        aws s3 cp "$DEST" "s3://$S3_BUCKET/$BACKUP_FILE"
    else
        echo "WARNING: aws CLI not found, skipping S3 upload"
    fi
fi

# Optional: keep only last N backups
KEEP_LAST="${KEEP_LAST:-30}"
ls -1t "$BACKUP_DIR"/marrow_*.db | tail -n +$((KEEP_LAST + 1)) | xargs -r rm -f

echo "==> Backup complete: $DEST"
