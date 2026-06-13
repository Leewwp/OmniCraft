use serde::{Deserialize, Serialize};

#[cfg(test)]
use serial_test::serial;

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

    #[test]
    fn test_detect_environment_with_custom_home() {
        let custom = if cfg!(windows) { "USERPROFILE" } else { "HOME" };
        let original = std::env::var(custom).ok();
        let test_path = "/tmp/tst_omnicraft_custom_home";

        std::env::set_var(custom, test_path);
        let result = detect_environment();

        // Restore before asserting so cleanup always runs
        match original {
            Some(val) => std::env::set_var(custom, val),
            None => std::env::remove_var(custom),
        }

        let env = result.expect("detect_environment should return Ok");
        assert!(
            env.home_dir.contains("tst_omnicraft_custom_home"),
            "home_dir should reflect the custom path, got: {}",
            env.home_dir,
        );
    }

    #[test]
    fn test_detect_environment_handles_empty_vars() {
        let home_key = if cfg!(windows) { "USERPROFILE" } else { "HOME" };
        let orig_home = std::env::var(home_key).ok();
        let orig_home_alt = if cfg!(windows) { std::env::var("HOME").ok() } else { None };
        let orig_appdata = std::env::var("APPDATA").ok();

        // Remove both HOME and USERPROFILE so detect_environment hits unwrap_or_default
        std::env::remove_var(home_key);
        if cfg!(windows) {
            std::env::remove_var("HOME");
        }
        std::env::remove_var("APPDATA");

        let result = detect_environment();

        // Restore before asserting
        match orig_home {
            Some(val) => std::env::set_var(home_key, val),
            None => std::env::remove_var(home_key),
        }
        if cfg!(windows) {
            match orig_home_alt {
                Some(val) => std::env::set_var("HOME", val),
                None => std::env::remove_var("HOME"),
            }
        }
        match orig_appdata {
            Some(val) => std::env::set_var("APPDATA", val),
            None => std::env::remove_var("APPDATA"),
        }

        assert!(result.is_ok(), "detect_environment should return Ok even with empty vars");
        let env = result.unwrap();
        // home_dir should fall back to empty string, not cause an error
        assert_eq!(
            env.home_dir, "",
            "home_dir should be empty when HOME/USERPROFILE are unset, got: {}",
            env.home_dir,
        );
    }

    #[test]
    #[serial]
    fn test_detect_environment_steam_paths_is_vec() {
        let result = detect_environment();
        assert!(result.is_ok(), "detect_environment should return Ok");
        let env = result.unwrap();
        // steam_paths should always be a Vec<String>, even if empty
        assert!(
            env.steam_paths.iter().all(|p| !p.is_empty()),
            "steam_paths entries should not be empty strings",
        );
        // Verify it's indeed a Vec by checking len() works and it's not an error
        let _len = env.steam_paths.len();
    }
}
