use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileOpResult {
    pub success: bool,
    pub message: String,
    pub path: Option<String>,
}

// ── Whitelist resolution ──────────────────────────────────────────────

fn resolve_whitelist_roots() -> Vec<PathBuf> {
    let mut roots = Vec::new();

    // $HOME/.omnicraft/**
    if let Ok(home) = std::env::var("HOME").or_else(|_| std::env::var("USERPROFILE")) {
        roots.push(PathBuf::from(&home).join(".omnicraft"));
    }

    // $APPDATA/omnicraft/**
    if let Ok(appdata) = std::env::var("APPDATA") {
        roots.push(PathBuf::from(&appdata).join("omnicraft"));
    }

    // $TEMP/omnicraft/**
    let temp = std::env::var("TEMP")
        .or_else(|_| std::env::var("TMP"))
        .or_else(|_| std::env::var("TMPDIR"));
    if let Ok(tmp) = temp {
        roots.push(PathBuf::from(&tmp).join("omnicraft"));
    }

    roots
}

fn is_within_whitelist(path: &Path) -> Result<PathBuf, String> {
    let canonical = path
        .canonicalize()
        .map_err(|e| format!("cannot resolve path '{}': {}", path.display(), e))?;

    for root in resolve_whitelist_roots() {
        let root_canonical = root.canonicalize();
        if let Ok(rc) = root_canonical {
            if canonical.starts_with(&rc) {
                return Ok(canonical);
            }
        }
        // If root doesn't exist yet, check prefix via path components
        if canonical.starts_with(&root) {
            return Ok(canonical);
        }
    }

    Err(format!(
        "path '{}' is outside allowed directories",
        path.display()
    ))
}

fn canonical_or_abs(path: &Path) -> PathBuf {
    path.canonicalize().unwrap_or_else(|_| path.to_path_buf())
}

// ── Tauri commands ────────────────────────────────────────────────────

#[tauri::command]
pub fn create_dir(dir_path: String) -> Result<FileOpResult, String> {
    let path = PathBuf::from(&dir_path);

    // If path doesn't exist, validate parent; if does exist, validate itself
    let check_path = if path.exists() {
        path.clone()
    } else {
        path.parent()
            .map(|p| p.to_path_buf())
            .unwrap_or_else(|| path.clone())
    };

    is_within_whitelist(&check_path)?;

    std::fs::create_dir_all(&path)
        .map_err(|e| format!("failed to create directory: {}", e))?;

    Ok(FileOpResult {
        success: true,
        message: "directory created".into(),
        path: Some(path.to_string_lossy().to_string()),
    })
}

#[tauri::command]
pub fn move_file(source: String, dest: String) -> Result<FileOpResult, String> {
    let src = PathBuf::from(&source);
    let dst = PathBuf::from(&dest);

    if !src.exists() {
        return Err(format!("source file does not exist: {}", src.display()));
    }

    is_within_whitelist(&src)?;
    is_within_whitelist(&dst)?;

    // Auto-backup before overwriting existing destination
    if dst.exists() {
        backup_file_internal(&dst)?;
    }

    // Ensure parent directory exists
    if let Some(parent) = dst.parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("failed to create target directory: {}", e))?;
    }

    std::fs::rename(&src, &dst)
        .map_err(|e| format!("failed to move file: {}", e))?;

    Ok(FileOpResult {
        success: true,
        message: "file moved".into(),
        path: Some(dst.to_string_lossy().to_string()),
    })
}

#[tauri::command]
pub async fn download_file(url: String, dest_path: String) -> Result<FileOpResult, String> {
    let dest = PathBuf::from(&dest_path);

    // Parent directory must be in whitelist
    let parent = dest
        .parent()
        .map(|p| p.to_path_buf())
        .unwrap_or_else(|| dest.clone());
    is_within_whitelist(&parent)?;

    // Ensure parent exists
    std::fs::create_dir_all(&parent)
        .map_err(|e| format!("failed to create target directory: {}", e))?;

    // Auto-backup if destination exists
    if dest.exists() {
        backup_file_internal(&dest)?;
    }

    let response = reqwest::get(&url)
        .await
        .map_err(|e| format!("download failed: {}", e))?;

    if !response.status().is_success() {
        return Err(format!("download failed with status {}", response.status()));
    }

    let bytes = response
        .bytes()
        .await
        .map_err(|e| format!("failed to read response: {}", e))?;

    std::fs::write(&dest, &bytes)
        .map_err(|e| format!("failed to write file: {}", e))?;

    Ok(FileOpResult {
        success: true,
        message: "file downloaded".into(),
        path: Some(dest.to_string_lossy().to_string()),
    })
}

#[tauri::command]
pub fn extract_archive(archive_path: String, dest_dir: String) -> Result<FileOpResult, String> {
    let archive = PathBuf::from(&archive_path);
    let dest = PathBuf::from(&dest_dir);

    if !archive.exists() {
        return Err(format!("archive does not exist: {}", archive.display()));
    }

    is_within_whitelist(&archive)?;
    is_within_whitelist(&dest)?;

    std::fs::create_dir_all(&dest)
        .map_err(|e| format!("failed to create extract directory: {}", e))?;

    let file = std::fs::File::open(&archive)
        .map_err(|e| format!("cannot open archive: {}", e))?;

    let mut zip = zip::ZipArchive::new(file)
        .map_err(|e| format!("invalid zip archive: {}", e))?;

    let dest_canonical = canonical_or_abs(&dest);

    for i in 0..zip.len() {
        let mut entry = zip.by_index(i).map_err(|e| format!("zip read error: {}", e))?;
        let entry_name = entry.name().to_string();

        // Zip-slip protection: ensure extracted path stays within dest
        let entry_path = dest_canonical.join(&entry_name);
        let entry_canonical = canonical_or_abs(&entry_path);
        if !entry_canonical.starts_with(&dest_canonical) {
            return Err(format!("zip entry '{}' would escape target directory", entry_name));
        }

        if entry.is_dir() {
            std::fs::create_dir_all(&entry_path)
                .map_err(|e| format!("failed to create dir in archive: {}", e))?;
        } else {
            if let Some(parent) = entry_path.parent() {
                std::fs::create_dir_all(parent)
                    .map_err(|e| format!("failed to create parent dir: {}", e))?;
            }
            let mut out = std::fs::File::create(&entry_path)
                .map_err(|e| format!("cannot create extracted file: {}", e))?;
            std::io::copy(&mut entry, &mut out)
                .map_err(|e| format!("extract write error: {}", e))?;
        }
    }

    Ok(FileOpResult {
        success: true,
        message: "archive extracted".into(),
        path: Some(dest.to_string_lossy().to_string()),
    })
}

#[tauri::command]
pub fn read_config(config_path: String) -> Result<String, String> {
    let path = PathBuf::from(&config_path);
    is_within_whitelist(&path)?;

    let ext = path
        .extension()
        .and_then(|e| e.to_str())
        .unwrap_or("");
    let allowed = ["ini", "cfg", "conf", "json", "toml", "yaml", "yml", "xml", "txt"];
    if !allowed.contains(&ext.to_lowercase().as_str()) {
        return Err(format!(
            "config file extension '.{}' not allowed; allowed: {}",
            ext,
            allowed.join(", ")
        ));
    }

    if !path.exists() {
        return Err(format!("config file not found: {}", path.display()));
    }

    std::fs::read_to_string(&path)
        .map_err(|e| format!("failed to read config: {}", e))
}

const CONFIG_MAX_SIZE: u64 = 5 * 1024 * 1024; // 5 MB

#[tauri::command]
pub fn write_config(config_path: String, content: String) -> Result<FileOpResult, String> {
    let path = PathBuf::from(&config_path);
    is_within_whitelist(&path)?;

    let ext = path
        .extension()
        .and_then(|e| e.to_str())
        .unwrap_or("");
    let allowed = ["ini", "cfg", "conf", "json", "toml", "yaml", "yml", "xml", "txt"];
    if !allowed.contains(&ext.to_lowercase().as_str()) {
        return Err(format!(
            "config file extension '.{}' not allowed; allowed: {}",
            ext,
            allowed.join(", ")
        ));
    }

    if content.len() as u64 > CONFIG_MAX_SIZE {
        return Err(format!(
            "config content too large ({} bytes, max {})",
            content.len(),
            CONFIG_MAX_SIZE
        ));
    }

    // Auto-backup before writing
    if path.exists() {
        backup_file_internal(&path)?;
    }

    // Ensure parent directory exists
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("failed to create parent dir: {}", e))?;
    }

    std::fs::write(&path, &content)
        .map_err(|e| format!("failed to write config: {}", e))?;

    Ok(FileOpResult {
        success: true,
        message: "config written".into(),
        path: Some(path.to_string_lossy().to_string()),
    })
}

/// Internal backup routine — copies the file to `.omnicraft_backup/`
/// with a timestamped filename. Not exposed as a Tauri command per CLAUDE.md:
/// "backup_file is system-auto only, forbid direct LLM invocation."
fn backup_file_internal(source: &Path) -> Result<(), String> {
    let parent = source
        .parent()
        .ok_or_else(|| "cannot determine parent dir for backup".to_string())?;

    let backup_dir = parent.join(".omnicraft_backup");
    std::fs::create_dir_all(&backup_dir)
        .map_err(|e| format!("cannot create backup dir: {}", e))?;

    let stem = source
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or("unnamed");
    let ext = source
        .extension()
        .and_then(|s| s.to_str())
        .map(|e| format!(".{}", e))
        .unwrap_or_default();

    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    let backup_name = format!("{}.{}{}.bak", stem, ts, ext);
    let backup_path = backup_dir.join(&backup_name);

    std::fs::copy(source, &backup_path)
        .map_err(|e| format!("backup failed: {}", e))?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    fn temp_dir() -> PathBuf {
        let dir = std::env::temp_dir().join("omnicraft").join("test");
        std::fs::create_dir_all(&dir).ok();
        dir
    }

    #[test]
    fn test_whitelist_allows_temp_omnicraft() {
        let dir = temp_dir();
        let result = is_within_whitelist(&dir);
        assert!(result.is_ok(), "temp omnicraft dir should be allowed: {:?}", result);
    }

    #[test]
    fn test_whitelist_rejects_system() {
        let result = is_within_whitelist(Path::new(r"C:\Windows\System32"));
        assert!(result.is_err());
    }

    #[test]
    fn test_create_dir_and_verify() {
        let dir = temp_dir().join("create_test");
        let _ = std::fs::remove_dir_all(&dir);
        let result = create_dir(dir.to_string_lossy().to_string());
        assert!(result.is_ok());
        assert!(dir.exists());
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_create_dir_outside_whitelist_rejected() {
        let result = create_dir(r"C:\Windows\System32\omnicraft-test".into());
        assert!(result.is_err());
    }

    #[test]
    fn test_move_file_with_whitelist() {
        let dir = temp_dir().join("move_test");
        std::fs::create_dir_all(&dir).ok();
        let src = dir.join("src.txt");
        let dst = dir.join("dst.txt");
        std::fs::write(&src, "hello").unwrap();

        let result = move_file(
            src.to_string_lossy().to_string(),
            dst.to_string_lossy().to_string(),
        );
        assert!(result.is_ok());
        assert!(dst.exists());
        assert!(!src.exists());

        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_read_config_allowed() {
        let dir = temp_dir().join("config_test");
        std::fs::create_dir_all(&dir).ok();
        let cfg = dir.join("test.json");
        std::fs::write(&cfg, r#"{"key": "value"}"#).unwrap();

        let result = read_config(cfg.to_string_lossy().to_string());
        assert!(result.is_ok());
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_read_config_rejects_disallowed_extension() {
        let dir = temp_dir().join("config_test2");
        std::fs::create_dir_all(&dir).ok();
        let cfg = dir.join("test.exe");
        std::fs::write(&cfg, "not an exe").unwrap();

        let result = read_config(cfg.to_string_lossy().to_string());
        assert!(result.is_err());
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_write_config_auto_backup() {
        let dir = temp_dir().join("write_test");
        std::fs::create_dir_all(&dir).ok();
        let cfg = dir.join("settings.json");
        std::fs::write(&cfg, r#"{"v": 1}"#).unwrap();

        let result = write_config(cfg.to_string_lossy().to_string(), r#"{"v": 2}"#.into());
        assert!(result.is_ok());

        // Check backup was created
        let backup_dir = dir.join(".omnicraft_backup");
        assert!(backup_dir.exists());
        let entries: Vec<_> = std::fs::read_dir(&backup_dir).unwrap().collect();
        assert!(!entries.is_empty());

        let _ = std::fs::remove_dir_all(&dir);
    }
}
