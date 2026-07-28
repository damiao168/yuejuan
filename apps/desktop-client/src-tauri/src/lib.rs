use serde::{Deserialize, Serialize};
use std::{fs, fs::OpenOptions, io::Write};
use tauri::{AppHandle, Manager};

const MAX_LOG_FILE_BYTES: u64 = 5 * 1024 * 1024;
const MAX_LOG_ID_BYTES: usize = 128;
const MAX_LOG_TIMESTAMP_BYTES: usize = 64;
const MAX_LOG_MESSAGE_BYTES: usize = 4 * 1024;
const MAX_LOG_CONTEXT_BYTES: usize = 32 * 1024;

#[derive(Serialize)]
struct CapabilityProbe {
    key: String,
    name: String,
    status: String,
    detail: String,
}

#[derive(Serialize)]
struct RuntimeDiagnostics {
    runtime: String,
    platform: String,
    #[serde(rename = "appVersion")]
    app_version: String,
    #[serde(rename = "logPath")]
    log_path: Option<String>,
}

#[derive(Deserialize, Serialize)]
struct LocalLogEntry {
    id: String,
    at: String,
    level: String,
    message: String,
    context: Option<String>,
}

impl LocalLogEntry {
    fn validate(&self) -> Result<(), String> {
        validate_required("id", &self.id, MAX_LOG_ID_BYTES)?;
        validate_required("at", &self.at, MAX_LOG_TIMESTAMP_BYTES)?;
        validate_required("message", &self.message, MAX_LOG_MESSAGE_BYTES)?;
        if !matches!(self.level.as_str(), "info" | "warning" | "error") {
            return Err("log level is invalid".into());
        }
        if self
            .context
            .as_ref()
            .is_some_and(|context| context.len() > MAX_LOG_CONTEXT_BYTES)
        {
            return Err("log context exceeds the size limit".into());
        }
        Ok(())
    }
}

fn validate_required(name: &str, value: &str, max_bytes: usize) -> Result<(), String> {
    if value.trim().is_empty() {
        return Err(format!("log {name} is required"));
    }
    if value.len() > max_bytes {
        return Err(format!("log {name} exceeds the size limit"));
    }
    Ok(())
}

#[tauri::command]
fn capability_statuses() -> Vec<CapabilityProbe> {
    vec![
        CapabilityProbe {
            key: "secure_config".into(),
            name: "本地加密配置存储".into(),
            status: "not_configured".into(),
            detail: "未配置/待接入 Stronghold、Windows DPAPI 或企业安全存储。当前只保留接口。"
                .into(),
        },
        CapabilityProbe {
            key: "local_cache".into(),
            name: "SQLite 本地缓存".into(),
            status: "not_configured".into(),
            detail: "未配置/待接入 SQLite schema、加密密钥和离线任务包缓存。".into(),
        },
        CapabilityProbe {
            key: "device_binding".into(),
            name: "设备绑定".into(),
            status: "not_configured".into(),
            detail: "未配置/待接入后端设备登记、吊销和绑定校验接口。".into(),
        },
        CapabilityProbe {
            key: "auto_update".into(),
            name: "自动更新".into(),
            status: "not_configured".into(),
            detail: "未配置/待接入内网更新源、签名校验和灰度策略。".into(),
        },
    ]
}

#[tauri::command]
fn runtime_diagnostics(app: AppHandle) -> RuntimeDiagnostics {
    let log_path = app
        .path()
        .app_log_dir()
        .ok()
        .map(|path| path.to_string_lossy().to_string());
    RuntimeDiagnostics {
        runtime: "tauri".into(),
        platform: std::env::consts::OS.into(),
        app_version: app.package_info().version.to_string(),
        log_path,
    }
}

#[tauri::command]
fn append_local_log(app: AppHandle, entry: LocalLogEntry) -> Result<(), String> {
    entry.validate()?;
    let log_dir = app.path().app_log_dir().map_err(|err| err.to_string())?;
    fs::create_dir_all(&log_dir).map_err(|err| err.to_string())?;
    let log_path = log_dir.join("client.log");
    if fs::symlink_metadata(&log_path)
        .map(|metadata| metadata.file_type().is_symlink())
        .unwrap_or(false)
    {
        return Err("refusing to write through a symbolic log path".into());
    }
    if fs::metadata(&log_path)
        .map(|metadata| metadata.len() >= MAX_LOG_FILE_BYTES)
        .unwrap_or(false)
    {
        let archived_path = log_dir.join("client.log.1");
        match fs::remove_file(&archived_path) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => return Err(error.to_string()),
        }
        fs::rename(&log_path, &archived_path).map_err(|err| err.to_string())?;
    }
    let mut file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(log_path)
        .map_err(|err| err.to_string())?;
    let line = serde_json::to_string(&entry).map_err(|err| err.to_string())?;
    writeln!(file, "{line}").map_err(|err| err.to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    if let Err(error) = tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            capability_statuses,
            runtime_diagnostics,
            append_local_log
        ])
        .run(tauri::generate_context!())
    {
        eprintln!("error while running EduGrade desktop client: {error}");
        std::process::exit(1);
    }
}
