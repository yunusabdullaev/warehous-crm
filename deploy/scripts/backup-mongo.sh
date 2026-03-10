#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════
#  Warehouse CRM — MongoDB Backup Script
# ═══════════════════════════════════════════════════════
#  Usage:   ./backup-mongo.sh
#  Crontab: 0 2 * * * /opt/wms/deploy/scripts/backup-mongo.sh
#  Systemd: see wms-backup.service + wms-backup.timer
# ═══════════════════════════════════════════════════════
set -euo pipefail

# ── Config (override via environment) ──
BACKUP_DIR="${BACKUP_DIR:-/opt/wms/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
MONGO_CONTAINER="${MONGO_CONTAINER:-wms-mongodb}"
MONGO_USER="${MONGO_USER:-wmsadmin}"
MONGO_PASSWORD="${MONGO_PASSWORD:-}"
MONGO_DB="${MONGO_DB:-warehouse_crm}"
S3_BUCKET="${S3_BUCKET:-}"
S3_ENDPOINT="${S3_ENDPOINT:-}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="wms_${MONGO_DB}_${TIMESTAMP}"
BACKUP_PATH="${BACKUP_DIR}/${BACKUP_NAME}"

# ── Logging ──
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"; }

log "Starting MongoDB backup: ${BACKUP_NAME}"

# ── Ensure backup directory exists ──
mkdir -p "${BACKUP_DIR}"

# ── Perform mongodump via docker exec ──
log "Running mongodump inside container '${MONGO_CONTAINER}'..."
docker exec "${MONGO_CONTAINER}" mongodump \
    --username="${MONGO_USER}" \
    --password="${MONGO_PASSWORD}" \
    --authenticationDatabase=admin \
    --db="${MONGO_DB}" \
    --archive \
    --gzip \
    > "${BACKUP_PATH}.gz"

BACKUP_SIZE=$(du -h "${BACKUP_PATH}.gz" | cut -f1)
log "Backup created: ${BACKUP_PATH}.gz (${BACKUP_SIZE})"

# ── Clean up old backups ──
log "Cleaning backups older than ${RETENTION_DAYS} days..."
DELETED=$(find "${BACKUP_DIR}" -name "wms_*.gz" -type f -mtime "+${RETENTION_DAYS}" -print -delete | wc -l)
log "Deleted ${DELETED} old backup(s)"

# ── Optional: Upload to S3 ──
if [ -n "${S3_BUCKET}" ]; then
    S3_FLAGS=""
    if [ -n "${S3_ENDPOINT}" ]; then
        S3_FLAGS="--endpoint-url ${S3_ENDPOINT}"
    fi
    log "Uploading to s3://${S3_BUCKET}/wms-backups/${BACKUP_NAME}.gz ..."
    aws s3 cp ${S3_FLAGS} \
        "${BACKUP_PATH}.gz" \
        "s3://${S3_BUCKET}/wms-backups/${BACKUP_NAME}.gz"
    log "S3 upload complete"
fi

# ── Summary ──
TOTAL_BACKUPS=$(find "${BACKUP_DIR}" -name "wms_*.gz" -type f | wc -l)
TOTAL_SIZE=$(du -sh "${BACKUP_DIR}" | cut -f1)
log "Backup complete. Total backups: ${TOTAL_BACKUPS}, Total size: ${TOTAL_SIZE}"
