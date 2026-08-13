use aes_gcm::{
    aead::{rand_core::RngCore, Aead, OsRng},
    Aes256Gcm, KeyInit, Nonce,
};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Write,
    path::{Path, PathBuf},
};
use tauri::{AppHandle, Manager};
use uuid::Uuid;
use zeroize::Zeroize;

const MAX_SPOOL_ASSET_BYTES: usize = 110 * 1024 * 1024;
const MAX_DRAFT_PAYLOAD_BYTES: usize = 4 * 1024 * 1024;
const DURABLE_CREDENTIAL_SERVICE: &str = "com.edugrade.enterprise.desktop";
const DURABLE_CREDENTIAL_ACCOUNT: &str = "desktop-offline-master-key-v1";

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SpoolAssetInput {
    pub filename: String,
    pub mime: String,
    pub bytes: Vec<u8>,
    pub exam_id: Option<String>,
    pub capture_batch_id: Option<String>,
    pub submission_id: Option<String>,
    pub page_no: Option<i64>,
    pub quality_checks: Option<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DurableQueueItem {
    pub id: String,
    pub title: String,
    pub kind: String,
    pub status: String,
    pub progress: u8,
    pub detail: String,
    pub updated_at: String,
    pub exam_id: Option<String>,
    pub exam_name: Option<String>,
    pub capture_batch_id: Option<String>,
    pub submission_id: Option<String>,
    pub page_no: Option<i64>,
    pub file_name: Option<String>,
    pub file_size: Option<i64>,
    pub content_type: Option<String>,
    pub file_asset_id: Option<String>,
    pub server_status: Option<String>,
    pub requires_reselect: Option<bool>,
    pub quality_checks: Option<Value>,
    pub local_asset_id: Option<String>,
    pub idempotency_key: Option<String>,
    pub retry_count: Option<i64>,
    pub confirmed_offset: Option<i64>,
    pub remote_upload_id: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DurableSpoolFile {
    pub filename: String,
    pub mime: String,
    pub bytes: Vec<u8>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DurableStoreStatus {
    pub ready: bool,
    pub database_path: String,
    pub spool_path: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct OfflineDraftEnvelope {
    pub task_id: String,
    pub anonymous_code: String,
    pub saved_at: String,
    pub expires_at: String,
    pub sync_status: String,
    pub sync_message: Option<String>,
}

pub fn status(app: AppHandle) -> Result<DurableStoreStatus, String> {
    let root = store_root(&app)?;
    let _ = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    Ok(DurableStoreStatus {
        ready: true,
        database_path: db_path(&root).to_string_lossy().into_owned(),
        spool_path: spool_path(&root).to_string_lossy().into_owned(),
    })
}

pub fn spool_local_asset(
    app: AppHandle,
    input: SpoolAssetInput,
) -> Result<DurableQueueItem, String> {
    validate_spool_input(&input)?;
    let root = store_root(&app)?;
    let key = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;

    let sha256 = sha256_hex(&input.bytes);
    let idempotency_key = format!(
        "scan-upload-v1:{}:{}:{}:{}:{}",
        sha256,
        input.exam_id.as_deref().unwrap_or(""),
        input.capture_batch_id.as_deref().unwrap_or(""),
        input.submission_id.as_deref().unwrap_or(""),
        input.page_no.unwrap_or_default()
    );
    if let Some(existing_id) = conn
        .query_row(
            "SELECT entity_id FROM sync_event WHERE operation = 'scan_upload' AND idempotency_key = ?1",
            params![idempotency_key],
            |row| row.get::<_, String>(0),
        )
        .optional()
        .map_err(sql_error)?
    {
        return read_queue_item(&conn, &key, &existing_id);
    }

    let id = Uuid::new_v4().to_string();
    let (encrypted_bytes, nonce) = encrypt_bytes(&key, &input.bytes)?;
    let file_path = spool_path(&root).join(format!("{id}.asset"));
    let file_path_text = file_path.to_string_lossy().into_owned();
    write_new_private_file(&file_path, &encrypted_bytes)?;
    let now = now_rfc3339();
    let mut item = DurableQueueItem {
        id: id.clone(),
        title: input.filename.clone(),
        kind: "scan_upload".to_string(),
        status: "pending".to_string(),
        progress: 0,
        detail: "已安全写入本地扫描队列，等待上传".to_string(),
        updated_at: now.clone(),
        exam_id: input.exam_id,
        exam_name: None,
        capture_batch_id: input.capture_batch_id,
        submission_id: input.submission_id,
        page_no: input.page_no,
        file_name: Some(input.filename),
        file_size: Some(input.bytes.len() as i64),
        content_type: Some(input.mime.clone()),
        file_asset_id: None,
        server_status: None,
        requires_reselect: Some(false),
        quality_checks: input.quality_checks,
        local_asset_id: Some(id.clone()),
        idempotency_key: Some(idempotency_key.clone()),
        retry_count: Some(0),
        confirmed_offset: Some(0),
        remote_upload_id: None,
    };
    let (payload_ciphertext, payload_nonce) = encrypt_json(&key, &item)?;
    let transaction = conn.unchecked_transaction().map_err(sql_error)?;
    transaction
        .execute(
            "INSERT INTO local_asset (id, sha256, size_bytes, mime, local_path, file_nonce, state, created_at, updated_at)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, 'queued', ?7, ?7)",
            params![
                id,
                sha256,
                input.bytes.len() as i64,
                input.mime,
                file_path_text,
                nonce,
                now,
            ],
        )
        .map_err(sql_error)?;
    transaction
        .execute(
            "INSERT INTO sync_event (id, operation, entity_id, idempotency_key, state, retry_count, last_error, payload_ciphertext, payload_nonce, created_at, updated_at)
             VALUES (?1, 'scan_upload', ?1, ?2, 'pending', 0, NULL, ?3, ?4, ?5, ?5)",
            params![idempotency_key, id, payload_ciphertext, payload_nonce, now],
        )
        .map_err(sql_error)?;
    transaction.commit().map_err(sql_error)?;
    item.idempotency_key = Some(idempotency_key);
    Ok(item)
}

pub fn list_durable_scan_queue(app: AppHandle) -> Result<Vec<DurableQueueItem>, String> {
    let root = store_root(&app)?;
    let key = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    // A process can stop after the server has confirmed one or more chunks but
    // before the next UI update.  Treat only the local, transient `uploading`
    // label as interrupted on restart; remote_upload_id and confirmed_offset
    // remain encrypted in the queue payload.  The next init call asks the
    // server for its authoritative offset and cannot resend confirmed chunks.
    recover_interrupted_uploads(&conn)?;
    let mut statement = conn
        .prepare(
            "SELECT entity_id FROM sync_event
             WHERE operation = 'scan_upload' AND state <> 'archived'
             ORDER BY updated_at DESC",
        )
        .map_err(sql_error)?;
    let ids = statement
        .query_map([], |row| row.get::<_, String>(0))
        .map_err(sql_error)?
        .collect::<Result<Vec<_>, _>>()
        .map_err(sql_error)?;
    ids.iter()
        .map(|id| read_queue_item(&conn, &key, id))
        .collect()
}

fn recover_interrupted_uploads(conn: &Connection) -> Result<(), String> {
    conn.execute(
        "UPDATE sync_event
         SET state = 'pending', updated_at = ?1
         WHERE operation = 'scan_upload' AND state = 'uploading'",
        params![now_rfc3339()],
    )
    .map_err(sql_error)?;
    conn.execute(
        "UPDATE local_asset
         SET state = 'queued', updated_at = ?1
         WHERE state = 'uploading'",
        params![now_rfc3339()],
    )
    .map_err(sql_error)?;
    Ok(())
}

pub fn persist_durable_scan_queue_item(
    app: AppHandle,
    mut item: DurableQueueItem,
) -> Result<(), String> {
    if item.kind != "scan_upload" {
        return Err("only scan_upload records can enter the durable scan queue".into());
    }
    let root = store_root(&app)?;
    let key = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let id = item
        .local_asset_id
        .clone()
        .unwrap_or_else(|| item.id.clone());
    let previous = read_queue_item(&conn, &key, &id)?;
    validate_transition(&previous.status, &item.status)?;
    item.id = id.clone();
    item.local_asset_id = Some(id.clone());
    item.idempotency_key = previous.idempotency_key.clone();
    item.retry_count = Some(
        previous.retry_count.unwrap_or_default()
            + i64::from(item.status == "failed" && previous.status != "failed"),
    );
    item.updated_at = now_rfc3339();
    let (payload_ciphertext, payload_nonce) = encrypt_json(&key, &item)?;
    let transaction = conn.unchecked_transaction().map_err(sql_error)?;
    transaction
        .execute(
            "UPDATE sync_event
             SET state = ?2, retry_count = ?3, last_error = ?4, payload_ciphertext = ?5, payload_nonce = ?6, updated_at = ?7
             WHERE entity_id = ?1 AND operation = 'scan_upload'",
            params![
                id,
                item.status,
                item.retry_count.unwrap_or_default(),
                if item.status == "failed" || item.status == "conflict" { Some(item.detail.as_str()) } else { None },
                payload_ciphertext,
                payload_nonce,
                item.updated_at,
            ],
        )
        .map_err(sql_error)?;
    if let Some(remote_id) = item.remote_upload_id.as_deref() {
        transaction
            .execute(
                "INSERT INTO upload_session (local_asset_id, remote_id, chunk_size, confirmed_offset, retry_count, last_error, updated_at)
                 VALUES (?1, ?2, NULL, ?3, ?4, ?5, ?6)
                 ON CONFLICT(local_asset_id) DO UPDATE SET remote_id = excluded.remote_id,
                   confirmed_offset = excluded.confirmed_offset, retry_count = excluded.retry_count,
                   last_error = excluded.last_error, updated_at = excluded.updated_at",
                params![
                    id,
                    remote_id,
                    item.confirmed_offset.unwrap_or_default(),
                    item.retry_count.unwrap_or_default(),
                    if item.status == "failed" || item.status == "conflict" { Some(item.detail.as_str()) } else { None },
                    item.updated_at,
                ],
            )
            .map_err(sql_error)?;
    }
    let asset_state = match item.status.as_str() {
        "succeeded" => "confirmed",
        "uploading" => "uploading",
        "conflict" => "conflict",
        "failed" => "failed",
        _ => "queued",
    };
    transaction
        .execute(
            "UPDATE local_asset
             SET state = ?2, updated_at = ?3, confirmed_at = CASE WHEN ?2 = 'confirmed' THEN ?3 ELSE confirmed_at END
             WHERE id = ?1",
            params![id, asset_state, item.updated_at],
        )
        .map_err(sql_error)?;
    transaction.commit().map_err(sql_error)
}

pub fn archive_durable_scan_queue_items(app: AppHandle, ids: Vec<String>) -> Result<(), String> {
    if ids.is_empty() {
        return Ok(());
    }
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let transaction = conn.unchecked_transaction().map_err(sql_error)?;
    for id in ids {
        let state = transaction
            .query_row(
                "SELECT state FROM sync_event WHERE operation = 'scan_upload' AND entity_id = ?1",
                params![id],
                |row| row.get::<_, String>(0),
            )
            .optional()
            .map_err(sql_error)?;
        if state.as_deref() != Some("succeeded") {
            return Err("only server-confirmed scan assets can be archived".into());
        }
        let now = now_rfc3339();
        transaction
            .execute(
                "UPDATE sync_event SET state = 'archived', updated_at = ?2 WHERE operation = 'scan_upload' AND entity_id = ?1",
                params![id, now],
            )
            .map_err(sql_error)?;
        transaction
            .execute(
                "UPDATE local_asset SET state = 'retained', updated_at = ?2 WHERE id = ?1",
                params![id, now],
            )
            .map_err(sql_error)?;
    }
    transaction.commit().map_err(sql_error)
}

pub fn read_durable_local_asset(
    app: AppHandle,
    local_asset_id: String,
) -> Result<DurableSpoolFile, String> {
    let root = store_root(&app)?;
    let key = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let (sha256, mime, local_path, nonce) = conn
        .query_row(
            "SELECT sha256, mime, local_path, file_nonce FROM local_asset WHERE id = ?1",
            params![local_asset_id],
            |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(1)?,
                    row.get::<_, String>(2)?,
                    row.get::<_, String>(3)?,
                ))
            },
        )
        .map_err(sql_error)?;
    let path = PathBuf::from(local_path);
    ensure_controlled_path(&root, &path)?;
    let encrypted = fs::read(&path).map_err(|error| error.to_string())?;
    let bytes = decrypt_bytes(&key, &nonce, &encrypted)?;
    if sha256_hex(&bytes) != sha256 {
        return Err(
            "local spool integrity check failed; source file is retained for recovery".into(),
        );
    }
    let item = read_queue_item(&conn, &key, &local_asset_id)?;
    Ok(DurableSpoolFile {
        filename: item.file_name.unwrap_or(item.title),
        mime,
        bytes,
    })
}

pub fn save_durable_draft(app: AppHandle, record: Value) -> Result<(), String> {
    if serde_json::to_vec(&record)
        .map_err(|error| error.to_string())?
        .len()
        > MAX_DRAFT_PAYLOAD_BYTES
    {
        return Err("offline draft exceeds the durable storage limit".into());
    }
    let metadata = draft_metadata(&record)?;
    let root = store_root(&app)?;
    let key = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let (payload, nonce) = encrypt_json(&key, &record)?;
    conn.execute(
        "INSERT INTO draft (task_id, anonymous_code, saved_at, expires_at, sync_status, sync_message, payload_ciphertext, payload_nonce, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?3)
         ON CONFLICT(task_id) DO UPDATE SET anonymous_code = excluded.anonymous_code, saved_at = excluded.saved_at,
           expires_at = excluded.expires_at, sync_status = excluded.sync_status, sync_message = excluded.sync_message,
           payload_ciphertext = excluded.payload_ciphertext, payload_nonce = excluded.payload_nonce, updated_at = excluded.updated_at",
        params![metadata.task_id, metadata.anonymous_code, metadata.saved_at, metadata.expires_at, metadata.sync_status, metadata.sync_message, payload, nonce],
    ).map_err(sql_error)?;
    Ok(())
}

pub fn list_durable_drafts(app: AppHandle) -> Result<Vec<OfflineDraftEnvelope>, String> {
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let mut statement = conn
        .prepare("SELECT task_id, anonymous_code, saved_at, expires_at, sync_status, sync_message FROM draft ORDER BY saved_at DESC")
        .map_err(sql_error)?;
    let rows = statement
        .query_map([], |row| {
            Ok(OfflineDraftEnvelope {
                task_id: row.get(0)?,
                anonymous_code: row.get(1)?,
                saved_at: row.get(2)?,
                expires_at: row.get(3)?,
                sync_status: row.get(4)?,
                sync_message: row.get(5)?,
            })
        })
        .map_err(sql_error)?
        .collect::<Result<Vec<_>, _>>()
        .map_err(sql_error);
    rows
}

pub fn load_durable_draft(app: AppHandle, task_id: String) -> Result<Option<Value>, String> {
    let root = store_root(&app)?;
    let key = master_key(&root)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let payload = conn
        .query_row(
            "SELECT payload_ciphertext, payload_nonce FROM draft WHERE task_id = ?1",
            params![task_id],
            |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?)),
        )
        .optional()
        .map_err(sql_error)?;
    payload
        .map(|(ciphertext, nonce)| decrypt_json(&key, &ciphertext, &nonce))
        .transpose()
}

pub fn update_durable_draft_status(
    app: AppHandle,
    task_id: String,
    sync_status: String,
    sync_message: Option<String>,
) -> Result<(), String> {
    validate_draft_status(&sync_status)?;
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    let rows = conn.execute(
        "UPDATE draft SET sync_status = ?2, sync_message = ?3, saved_at = ?4, updated_at = ?4 WHERE task_id = ?1",
        params![task_id, sync_status, sync_message, now_rfc3339()],
    ).map_err(sql_error)?;
    if rows != 1 {
        return Err("offline draft was not found".into());
    }
    Ok(())
}

pub fn purge_expired_durable_drafts(app: AppHandle, now: String) -> Result<usize, String> {
    let conn = open_connection(&app)?;
    initialize_schema(&conn)?;
    conn.execute("DELETE FROM draft WHERE expires_at <= ?1", params![now])
        .map_err(sql_error)
}

fn open_connection(app: &AppHandle) -> Result<Connection, String> {
    let root = store_root(app)?;
    Connection::open(db_path(&root)).map_err(sql_error)
}

fn initialize_schema(conn: &Connection) -> Result<(), String> {
    conn.execute_batch(
        "PRAGMA journal_mode = WAL;
         PRAGMA foreign_keys = ON;
         CREATE TABLE IF NOT EXISTS local_asset (
           id TEXT PRIMARY KEY,
           sha256 TEXT NOT NULL,
           size_bytes INTEGER NOT NULL,
           mime TEXT NOT NULL,
           local_path TEXT NOT NULL UNIQUE,
           file_nonce TEXT NOT NULL,
           state TEXT NOT NULL,
           created_at TEXT NOT NULL,
           updated_at TEXT NOT NULL,
           confirmed_at TEXT,
           retention_until TEXT
         );
         CREATE TABLE IF NOT EXISTS scan_batch_local (
           id TEXT PRIMARY KEY,
           exam_id TEXT,
           submission_id TEXT,
           state TEXT NOT NULL,
           created_at TEXT NOT NULL,
           updated_at TEXT NOT NULL
         );
         CREATE TABLE IF NOT EXISTS upload_session (
           local_asset_id TEXT PRIMARY KEY REFERENCES local_asset(id) ON DELETE RESTRICT,
           remote_id TEXT,
           chunk_size INTEGER,
           confirmed_offset INTEGER NOT NULL DEFAULT 0,
           retry_count INTEGER NOT NULL DEFAULT 0,
           last_error TEXT,
           updated_at TEXT NOT NULL
         );
         CREATE TABLE IF NOT EXISTS upload_chunk (
           upload_session_asset_id TEXT NOT NULL REFERENCES upload_session(local_asset_id) ON DELETE RESTRICT,
           offset INTEGER NOT NULL,
           size_bytes INTEGER NOT NULL,
           sha256 TEXT NOT NULL,
           state TEXT NOT NULL,
           created_at TEXT NOT NULL,
           PRIMARY KEY(upload_session_asset_id, offset)
         );
         CREATE TABLE IF NOT EXISTS sync_event (
           id TEXT PRIMARY KEY,
           operation TEXT NOT NULL,
           entity_id TEXT NOT NULL REFERENCES local_asset(id) ON DELETE RESTRICT,
           idempotency_key TEXT NOT NULL UNIQUE,
           state TEXT NOT NULL,
           retry_count INTEGER NOT NULL DEFAULT 0,
           last_error TEXT,
           payload_ciphertext TEXT NOT NULL,
           payload_nonce TEXT NOT NULL,
           created_at TEXT NOT NULL,
           updated_at TEXT NOT NULL
         );
         CREATE INDEX IF NOT EXISTS idx_sync_event_pending ON sync_event(operation, state, updated_at);
         CREATE TABLE IF NOT EXISTS draft (
           task_id TEXT PRIMARY KEY,
           anonymous_code TEXT NOT NULL,
           saved_at TEXT NOT NULL,
           expires_at TEXT NOT NULL,
           sync_status TEXT NOT NULL,
           sync_message TEXT,
           payload_ciphertext TEXT NOT NULL,
           payload_nonce TEXT NOT NULL,
           updated_at TEXT NOT NULL
         );",
    ).map_err(sql_error)
}

fn read_queue_item(
    conn: &Connection,
    key: &[u8; 32],
    id: &str,
) -> Result<DurableQueueItem, String> {
    let (payload, nonce, state, retry_count) = conn
        .query_row(
            "SELECT payload_ciphertext, payload_nonce, state, retry_count
             FROM sync_event WHERE operation = 'scan_upload' AND entity_id = ?1",
            params![id],
            |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(1)?,
                    row.get::<_, String>(2)?,
                    row.get::<_, i64>(3)?,
                ))
            },
        )
        .map_err(sql_error)?;
    let mut item: DurableQueueItem = decrypt_json(key, &payload, &nonce)?;
    item.id = id.to_string();
    item.local_asset_id = Some(id.to_string());
    item.status = state;
    item.retry_count = Some(retry_count);
    Ok(item)
}

fn validate_spool_input(input: &SpoolAssetInput) -> Result<(), String> {
    if input.filename.trim().is_empty() || input.filename.len() > 512 {
        return Err("spool filename is invalid".into());
    }
    if input.mime.trim().is_empty() || input.mime.len() > 256 {
        return Err("spool MIME type is invalid".into());
    }
    if input.bytes.is_empty() || input.bytes.len() > MAX_SPOOL_ASSET_BYTES {
        return Err("spool asset size is invalid".into());
    }
    if input.page_no.is_some_and(|page| page <= 0) {
        return Err("spool page number must be positive".into());
    }
    Ok(())
}

fn validate_transition(previous: &str, next: &str) -> Result<(), String> {
    let allowed = matches!(
        (previous, next),
        ("pending", "pending" | "uploading" | "failed" | "conflict")
            | (
                "uploading",
                "uploading" | "pending" | "failed" | "succeeded" | "conflict"
            )
            | ("failed", "failed" | "pending" | "uploading" | "conflict")
            | ("conflict", "conflict" | "pending" | "failed")
            | ("succeeded", "succeeded")
    );
    if allowed {
        Ok(())
    } else {
        Err(format!(
            "invalid durable queue transition: {previous} -> {next}"
        ))
    }
}

fn validate_draft_status(status: &str) -> Result<(), String> {
    if matches!(
        status,
        "draft" | "syncing" | "synced" | "failed" | "conflict"
    ) {
        Ok(())
    } else {
        Err("offline draft status is invalid".into())
    }
}

struct DraftMetadata {
    task_id: String,
    anonymous_code: String,
    saved_at: String,
    expires_at: String,
    sync_status: String,
    sync_message: Option<String>,
}

fn draft_metadata(value: &Value) -> Result<DraftMetadata, String> {
    let object = value.as_object().ok_or("offline draft must be an object")?;
    let required = |name: &str| {
        object
            .get(name)
            .and_then(Value::as_str)
            .filter(|value| !value.trim().is_empty())
            .map(str::to_owned)
            .ok_or_else(|| format!("offline draft {name} is required"))
    };
    let metadata = DraftMetadata {
        task_id: required("taskId")?,
        anonymous_code: required("anonymousCode")?,
        saved_at: required("savedAt")?,
        expires_at: required("expiresAt")?,
        sync_status: required("syncStatus")?,
        sync_message: object
            .get("syncMessage")
            .and_then(Value::as_str)
            .map(str::to_owned),
    };
    validate_draft_status(&metadata.sync_status)?;
    Ok(metadata)
}

fn encrypt_json<T: Serialize>(key: &[u8; 32], value: &T) -> Result<(String, String), String> {
    let plaintext = serde_json::to_vec(value).map_err(|error| error.to_string())?;
    let (ciphertext, nonce) = encrypt_bytes(key, &plaintext)?;
    Ok((BASE64.encode(ciphertext), nonce))
}

fn decrypt_json<T: for<'de> Deserialize<'de>>(
    key: &[u8; 32],
    ciphertext: &str,
    nonce: &str,
) -> Result<T, String> {
    let ciphertext = BASE64
        .decode(ciphertext)
        .map_err(|_| "durable encrypted payload is invalid")?;
    let plaintext = decrypt_bytes(key, nonce, &ciphertext)?;
    serde_json::from_slice(&plaintext)
        .map_err(|_| "durable encrypted payload cannot be decoded".into())
}

fn encrypt_bytes(key: &[u8; 32], plaintext: &[u8]) -> Result<(Vec<u8>, String), String> {
    let cipher = Aes256Gcm::new_from_slice(key).map_err(|_| "durable encryption key is invalid")?;
    let mut nonce = [0_u8; 12];
    OsRng.fill_bytes(&mut nonce);
    let ciphertext = cipher
        .encrypt(Nonce::from_slice(&nonce), plaintext)
        .map_err(|_| "durable payload encryption failed")?;
    Ok((ciphertext, BASE64.encode(nonce)))
}

fn decrypt_bytes(key: &[u8; 32], nonce: &str, ciphertext: &[u8]) -> Result<Vec<u8>, String> {
    let nonce = BASE64
        .decode(nonce)
        .map_err(|_| "durable payload nonce is invalid")?;
    if nonce.len() != 12 {
        return Err("durable payload nonce has an invalid length".into());
    }
    let cipher = Aes256Gcm::new_from_slice(key).map_err(|_| "durable encryption key is invalid")?;
    cipher
        .decrypt(Nonce::from_slice(&nonce), ciphertext)
        .map_err(|_| {
            "durable payload cannot be decrypted; secure local key may be unavailable or corrupted"
                .into()
        })
}

#[cfg(windows)]
fn master_key(root: &Path) -> Result<[u8; 32], String> {
    let entry = keyring::Entry::new(DURABLE_CREDENTIAL_SERVICE, DURABLE_CREDENTIAL_ACCOUNT)
        .map_err(|error| error.to_string())?;
    match entry.get_password() {
        Ok(mut stored) => {
            let decoded = BASE64
                .decode(&stored)
                .map_err(|_| "durable master key is corrupt")?;
            stored.zeroize();
            decoded
                .try_into()
                .map_err(|_| "durable master key has an invalid length".to_string())
        }
        Err(keyring::Error::NoEntry) => {
            // Never replace a missing credential with a fresh key when a
            // previous station store exists.  Doing so would make queued
            // scans look recoverable until a later decrypt fails, and could
            // tempt an operator to discard the only encrypted source copy.
            if durable_data_exists(root)? {
                return Err("durable master key is missing while encrypted local data exists; recovery requires the original Windows Credential Manager entry and the client will not create a replacement key".into());
            }
            let mut key = [0_u8; 32];
            OsRng.fill_bytes(&mut key);
            let mut encoded = BASE64.encode(key);
            let result = entry
                .set_password(&encoded)
                .map_err(|error| error.to_string());
            encoded.zeroize();
            result.map(|()| key)
        }
        Err(error) => Err(format!(
            "cannot access Windows Credential Manager for the durable local key: {error}"
        )),
    }
}

#[cfg(not(windows))]
fn master_key(_root: &Path) -> Result<[u8; 32], String> {
    Err("durable local storage requires Windows Credential Manager and will not fall back to plaintext storage".into())
}

// This is intentionally conservative.  A missing key must not be treated as
// a first launch if either the SQLite store or a controlled spool asset is
// already present.  It is safer to require recovery of the Credential Manager
// entry than to create a key that can never decrypt the retained evidence.
fn durable_data_exists(root: &Path) -> Result<bool, String> {
    let database = db_path(root);
    if database.exists() {
        return Ok(true);
    }
    let spool = spool_path(root);
    let mut entries = fs::read_dir(&spool).map_err(|error| error.to_string())?;
    Ok(entries
        .next()
        .transpose()
        .map_err(|error| error.to_string())?
        .is_some())
}

fn store_root(app: &AppHandle) -> Result<PathBuf, String> {
    let root = app
        .path()
        .app_data_dir()
        .map_err(|error| error.to_string())?
        .join("durable-store-v1");
    fs::create_dir_all(spool_path(&root)).map_err(|error| error.to_string())?;
    reject_symbolic_path(&root)?;
    reject_symbolic_path(&spool_path(&root))?;
    Ok(root)
}

fn db_path(root: &Path) -> PathBuf {
    root.join("offline.sqlite3")
}
fn spool_path(root: &Path) -> PathBuf {
    root.join("spool")
}

fn ensure_controlled_path(root: &Path, path: &Path) -> Result<(), String> {
    let canonical_root = root.canonicalize().map_err(|error| error.to_string())?;
    let canonical_path = path.canonicalize().map_err(|error| error.to_string())?;
    if !canonical_path.starts_with(spool_path(&canonical_root)) {
        return Err("refusing to access a spool path outside the durable store".into());
    }
    reject_symbolic_path(&canonical_path)
}

fn write_new_private_file(path: &Path, bytes: &[u8]) -> Result<(), String> {
    reject_symbolic_path(path)?;
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(path)
        .map_err(|error| error.to_string())?;
    file.write_all(bytes).map_err(|error| error.to_string())?;
    file.sync_all().map_err(|error| error.to_string())
}

fn reject_symbolic_path(path: &Path) -> Result<(), String> {
    if fs::symlink_metadata(path)
        .map(|metadata| metadata.file_type().is_symlink())
        .unwrap_or(false)
    {
        return Err("refusing to access a symbolic durable storage path".into());
    }
    Ok(())
}

fn sha256_hex(bytes: &[u8]) -> String {
    let digest = Sha256::digest(bytes);
    digest.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn now_rfc3339() -> String {
    chrono::Utc::now().to_rfc3339()
}

fn sql_error(error: rusqlite::Error) -> String {
    error.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encrypted_payload_requires_the_same_key_and_nonce() {
        let key = [7_u8; 32];
        let value = serde_json::json!({"answer": "敏感草稿", "score": 8});
        let (ciphertext, nonce) = encrypt_json(&key, &value).expect("encrypt");
        assert!(!ciphertext.contains("敏感草稿"));
        let decoded: Value = decrypt_json(&key, &ciphertext, &nonce).expect("decrypt");
        assert_eq!(decoded, value);
        assert!(decrypt_json::<Value>(&[8_u8; 32], &ciphertext, &nonce).is_err());
    }

    #[test]
    fn confirmed_assets_are_immutable_from_the_sync_queue() {
        assert!(validate_transition("pending", "uploading").is_ok());
        assert!(validate_transition("uploading", "succeeded").is_ok());
        assert!(validate_transition("succeeded", "failed").is_err());
        assert!(validate_transition("conflict", "pending").is_ok());
    }

    #[test]
    fn restart_recovers_only_incomplete_uploads_and_never_reopens_confirmed_assets() {
        let conn = Connection::open_in_memory().expect("in-memory SQLite");
        initialize_schema(&conn).expect("schema");
        let now = "2026-08-12T00:00:00Z";
        for (id, state) in [
            ("asset-uploading", "uploading"),
            ("asset-confirmed", "confirmed"),
        ] {
            conn.execute(
                "INSERT INTO local_asset (id, sha256, size_bytes, mime, local_path, file_nonce, state, created_at, updated_at)
                 VALUES (?1, 'hash', 1, 'application/pdf', ?2, 'nonce', ?3, ?4, ?4)",
                params![id, format!("C:/spool/{id}.asset"), state, now],
            )
            .expect("asset");
        }
        for (id, state) in [
            ("asset-uploading", "uploading"),
            ("asset-confirmed", "succeeded"),
        ] {
            conn.execute(
                "INSERT INTO sync_event (id, operation, entity_id, idempotency_key, state, retry_count, last_error, payload_ciphertext, payload_nonce, created_at, updated_at)
                 VALUES (?2, 'scan_upload', ?1, ?3, ?4, 0, NULL, 'ciphertext', 'nonce', ?5, ?5)",
                params![id, format!("event-{id}"), format!("key-{id}"), state, now],
            )
            .expect("event");
        }

        recover_interrupted_uploads(&conn).expect("recover interrupted upload");

        let queue_state = |id: &str| -> String {
            conn.query_row(
                "SELECT state FROM sync_event WHERE entity_id = ?1",
                params![id],
                |row| row.get(0),
            )
            .expect("queue state")
        };
        let asset_state = |id: &str| -> String {
            conn.query_row(
                "SELECT state FROM local_asset WHERE id = ?1",
                params![id],
                |row| row.get(0),
            )
            .expect("asset state")
        };
        assert_eq!(queue_state("asset-uploading"), "pending");
        assert_eq!(asset_state("asset-uploading"), "queued");
        assert_eq!(queue_state("asset-confirmed"), "succeeded");
        assert_eq!(asset_state("asset-confirmed"), "confirmed");
    }

    #[test]
    fn recovery_keeps_a_500_page_spool_and_the_interrupted_page_checkpoint() {
        // This is a station-store fixture, rather than a scanner test.  It
        // models a process kill after page 173 has been accepted through a
        // remote upload offset but before the desktop can continue the batch.
        // The encrypted queue is the durable source of truth on restart.
        let conn = Connection::open_in_memory().expect("in-memory SQLite");
        initialize_schema(&conn).expect("schema");
        let key = [29_u8; 32];
        let now = "2026-08-13T00:00:00Z";

        for page_no in 1_i64..=500 {
            let id = format!("page-{page_no}");
            let status = match page_no {
                1..=172 => "succeeded",
                173 => "uploading",
                _ => "pending",
            };
            let asset_state = match status {
                "succeeded" => "confirmed",
                "uploading" => "uploading",
                _ => "queued",
            };
            let checkpoint = DurableQueueItem {
                id: id.clone(),
                title: format!("answer-page-{page_no}.pdf"),
                kind: "scan_upload".to_string(),
                status: status.to_string(),
                progress: if page_no <= 172 { 100 } else { 0 },
                detail: "fixture".to_string(),
                updated_at: now.to_string(),
                exam_id: Some("exam-500".to_string()),
                exam_name: None,
                capture_batch_id: Some("batch-500".to_string()),
                submission_id: None,
                page_no: Some(page_no),
                file_name: Some(format!("answer-page-{page_no}.pdf")),
                file_size: Some(4096),
                content_type: Some("application/pdf".to_string()),
                file_asset_id: None,
                server_status: None,
                requires_reselect: Some(false),
                quality_checks: None,
                local_asset_id: Some(id.clone()),
                idempotency_key: Some(format!("stable-page-{page_no}")),
                retry_count: Some(0),
                // Page 173 was confirmed remotely to this exact boundary
                // before the simulated kill.  It must survive recovery.
                confirmed_offset: (page_no == 173).then_some(1_730_000),
                remote_upload_id: (page_no == 173).then_some("remote-page-173".to_string()),
            };
            let (payload, nonce) =
                encrypt_json(&key, &checkpoint).expect("encrypt queue checkpoint");
            conn.execute(
                "INSERT INTO local_asset (id, sha256, size_bytes, mime, local_path, file_nonce, state, created_at, updated_at)
                 VALUES (?1, ?2, 4096, 'application/pdf', ?3, 'nonce', ?4, ?5, ?5)",
                params![
                    id,
                    format!("hash-{page_no}"),
                    format!("C:/spool/page-{page_no}.asset"),
                    asset_state,
                    now,
                ],
            )
            .expect("local asset");
            conn.execute(
                "INSERT INTO sync_event (id, operation, entity_id, idempotency_key, state, retry_count, last_error, payload_ciphertext, payload_nonce, created_at, updated_at)
                 VALUES (?1, 'scan_upload', ?1, ?2, ?3, 0, NULL, ?4, ?5, ?6, ?6)",
                params![
                    checkpoint.id,
                    checkpoint.idempotency_key.as_deref().expect("idempotency key"),
                    status,
                    payload,
                    nonce,
                    now,
                ],
            )
            .expect("sync event");
        }
        conn.execute(
            "INSERT INTO upload_session (local_asset_id, remote_id, chunk_size, confirmed_offset, retry_count, last_error, updated_at)
             VALUES ('page-173', 'remote-page-173', 65536, 1730000, 0, NULL, ?1)",
            params![now],
        )
        .expect("upload session");

        recover_interrupted_uploads(&conn).expect("recover interrupted upload");

        let queue_count: i64 = conn
            .query_row(
                "SELECT COUNT(*) FROM sync_event WHERE operation = 'scan_upload' AND state <> 'archived'",
                [],
                |row| row.get(0),
            )
            .expect("queue count");
        let pending_count: i64 = conn
            .query_row(
                "SELECT COUNT(*) FROM sync_event WHERE operation = 'scan_upload' AND state = 'pending'",
                [],
                |row| row.get(0),
            )
            .expect("pending count");
        assert_eq!(queue_count, 500, "a restart must not drop queued pages");
        assert_eq!(
            pending_count, 328,
            "the interrupted page returns to pending with untouched later pages"
        );

        let (payload, nonce): (String, String) = conn
            .query_row(
                "SELECT payload_ciphertext, payload_nonce FROM sync_event WHERE entity_id = 'page-173'",
                [],
                |row| Ok((row.get(0)?, row.get(1)?)),
            )
            .expect("checkpoint payload");
        let restored: DurableQueueItem =
            decrypt_json(&key, &payload, &nonce).expect("restore checkpoint");
        assert_eq!(
            restored.remote_upload_id.as_deref(),
            Some("remote-page-173")
        );
        assert_eq!(restored.confirmed_offset, Some(1_730_000));
        let resumed_state: String = conn
            .query_row(
                "SELECT state FROM sync_event WHERE entity_id = 'page-173'",
                [],
                |row| row.get(0),
            )
            .expect("recovered queue state");
        let (remote_id, confirmed_offset): (String, i64) = conn
            .query_row(
                "SELECT remote_id, confirmed_offset FROM upload_session WHERE local_asset_id = 'page-173'",
                [],
                |row| Ok((row.get(0)?, row.get(1)?)),
            )
            .expect("upload session checkpoint");
        assert_eq!(resumed_state, "pending");
        assert_eq!(remote_id, "remote-page-173");
        assert_eq!(confirmed_offset, 1_730_000);
    }

    #[test]
    fn draft_metadata_rejects_unknown_sync_states() {
        let value = serde_json::json!({
            "taskId": "task-1",
            "anonymousCode": "A001",
            "savedAt": "2026-08-11T00:00:00Z",
            "expiresAt": "2026-08-18T00:00:00Z",
            "syncStatus": "not-a-state"
        });
        assert!(draft_metadata(&value).is_err());
    }

    #[test]
    fn an_existing_station_store_is_not_treated_as_first_launch() {
        let root = std::env::temp_dir().join(format!("edugrade-durable-store-{}", Uuid::new_v4()));
        fs::create_dir_all(spool_path(&root)).expect("create controlled spool");
        assert!(!durable_data_exists(&root).expect("empty station store"));

        fs::write(
            spool_path(&root).join("retained.asset"),
            b"encrypted source",
        )
        .expect("write retained source");
        assert!(durable_data_exists(&root).expect("retained spool counts as existing data"));

        fs::remove_file(spool_path(&root).join("retained.asset")).expect("remove fixture");
        fs::write(db_path(&root), b"SQLite format 3\0").expect("write station database marker");
        assert!(durable_data_exists(&root).expect("database counts as existing data"));
        fs::remove_dir_all(root).expect("remove fixture root");
    }
}
