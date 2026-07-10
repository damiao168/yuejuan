use serde::{Deserialize, Serialize};
use std::{fs, fs::OpenOptions, io::Write};
use tauri::{AppHandle, Manager};

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

#[tauri::command]
fn capability_statuses() -> Vec<CapabilityProbe> {
    vec![
        CapabilityProbe {
            key: "secure_config".into(),
            name: "本地加密配置存储".into(),
            status: "not_configured".into(),
            detail: "未配置/待接入 Stronghold、Windows DPAPI 或企业安全存储。当前只保留接口。".into(),
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
    let log_dir = app.path().app_log_dir().map_err(|err| err.to_string())?;
    fs::create_dir_all(&log_dir).map_err(|err| err.to_string())?;
    let mut file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(log_dir.join("client.log"))
        .map_err(|err| err.to_string())?;
    let line = serde_json::to_string(&entry).map_err(|err| err.to_string())?;
    writeln!(file, "{line}").map_err(|err| err.to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            capability_statuses,
            runtime_diagnostics,
            append_local_log
        ])
        .run(tauri::generate_context!())
        .expect("error while running EduGrade desktop client");
}
