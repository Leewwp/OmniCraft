use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EnvInfo {
    pub steam_paths: Vec<String>,
    pub platform: String,
    pub home_dir: String,
    pub appdata_dir: String,
}

#[tauri::command]
pub fn detect_environment() -> Result<EnvInfo, String> {
    let home = std::env::var("USERPROFILE")
        .or_else(|_| std::env::var("HOME"))
        .unwrap_or_default();

    let appdata = std::env::var("APPDATA")
        .or_else(|_| std::env::var("HOME").map(|h| format!("{}/.config", h)))
        .unwrap_or_default();

    Ok(EnvInfo {
        steam_paths: detect_steam_paths(),
        platform: std::env::consts::OS.to_string(),
        home_dir: home,
        appdata_dir: appdata,
    })
}

#[cfg(target_os = "windows")]
fn detect_steam_paths() -> Vec<String> {
    let mut paths = Vec::new();

    let candidates = [
        r"C:\Program Files (x86)\Steam",
        r"C:\Program Files\Steam",
        r"D:\Steam",
        r"E:\Steam",
    ];

    for candidate in &candidates {
        if std::path::Path::new(candidate).exists() {
            paths.push(candidate.to_string());
        }
    }

    if let Ok(output) = std::process::Command::new("reg")
        .args([
            "query",
            r"HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\Valve\Steam",
            "/v",
            "InstallPath",
        ])
        .output()
    {
        let stdout = String::from_utf8_lossy(&output.stdout);
        for line in stdout.lines() {
            if let Some(pos) = line.find("REG_SZ") {
                let p = line[pos + 6..].trim().to_string();
                if !p.is_empty() && std::path::Path::new(&p).exists() && !paths.contains(&p) {
                    paths.push(p);
                }
            }
        }
    }

    paths
}

#[cfg(target_os = "macos")]
fn detect_steam_paths() -> Vec<String> {
    let mut paths = Vec::new();
    let home = std::env::var("HOME").unwrap_or_default();
    let steam_dir = std::path::PathBuf::from(&home)
        .join("Library")
        .join("Application Support")
        .join("Steam");
    if steam_dir.exists() {
        paths.push(steam_dir.to_string_lossy().to_string());
    }
    paths
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
fn detect_steam_paths() -> Vec<String> {
    let mut paths = Vec::new();
    let home = std::env::var("HOME").unwrap_or_default();
    let steam_dir = std::path::PathBuf::from(&home).join(".steam").join("steam");
    if steam_dir.exists() {
        paths.push(steam_dir.to_string_lossy().to_string());
    }
    let flatpak = std::path::PathBuf::from(&home)
        .join(".var")
        .join("app")
        .join("com.valvesoftware.Steam");
    if flatpak.exists() {
        paths.push(flatpak.to_string_lossy().to_string());
    }
    paths
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_detect_environment_ok() {
        let result = detect_environment();
        assert!(result.is_ok());
        let env = result.unwrap();
        assert_eq!(env.platform, std::env::consts::OS);
    }
}
