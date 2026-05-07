mod commands;
mod url_scheme;

use std::sync::Mutex;
use tauri::Manager;
use url_scheme::UrlSchemeParams;

/// Stores deploy parameters received via URL scheme (omnicraft://deploy?...)
struct DeployState(Mutex<Option<UrlSchemeParams>>);

#[tauri::command]
fn get_deploy_params(state: tauri::State<DeployState>) -> Result<Option<UrlSchemeParams>, String> {
    match state.0.lock() {
        Ok(guard) => Ok(guard.clone()),
        Err(e) => Err(e.to_string()),
    }
}

#[cfg(target_os = "windows")]
fn register_url_scheme() {
    use std::process::Command;

    let target = r"HKEY_CURRENT_USER\Software\Classes\omnicraft";
    let result = Command::new("reg")
        .args(["add", target, "/ve", "/d", "URL:OmniCraft Protocol", "/f"])
        .output();

    if let Err(e) = &result {
        eprintln!("Warning: Failed to register URL scheme base: {}", e);
    }

    let icon_path = r"C:\Program Files\OmniCraft\icon.ico";
    let _ = Command::new("reg")
        .args([target, r"\DefaultIcon", "/ve", "/d", icon_path, "/f"])
        .output();

    let exe_path = std::env::current_exe()
        .unwrap_or_else(|_| std::path::PathBuf::from(r"C:\Program Files\OmniCraft\OmniCraft.exe"));

    let open_cmd = format!(
        "\"{}\" \"%1\"",
        exe_path.to_string_lossy().replace('\\', "\\\\")
    );
    let url_protocol = format!("{}\\shell\\open\\command", target);
    let result = Command::new("reg")
        .args([&url_protocol, "/ve", "/d", &open_cmd, "/f"])
        .output();

    match result {
        Ok(output) if output.status.success() => {
            println!("URL scheme 'omnicraft://' registered successfully");
        }
        Ok(output) => {
            let stderr = String::from_utf8_lossy(&output.stderr);
            eprintln!("Warning: Failed to register URL scheme handler: {}", stderr);
        }
        Err(e) => {
            eprintln!("Warning: Failed to execute reg command: {}", e);
        }
    }
}

#[cfg(target_os = "macos")]
fn register_url_scheme() {
    // macOS URL scheme registration is handled via Info.plist CFBundleURLTypes
    // configured in tauri.conf.json bundle section.
    println!("URL scheme registration handled via Info.plist (CFBundleURLTypes)");
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
fn register_url_scheme() {
    // Linux: URL scheme registered via .desktop file MimeType
    println!("URL scheme registration handled via .desktop file");
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // Register URL scheme on startup (Windows only — macOS uses Info.plist)
    register_url_scheme();

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .plugin(tauri_plugin_notification::init())
        .manage(DeployState(Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![
            get_deploy_params,
            commands::security::verify_script_signature,
            commands::file_ops::download_file,
            commands::file_ops::extract_archive,
            commands::file_ops::move_file,
            commands::file_ops::create_dir,
            commands::file_ops::read_config,
            commands::file_ops::write_config,
            commands::env_detect::detect_environment,
        ])
        .setup(|app| {
            // Parse URL scheme arguments passed via command line
            let args: Vec<String> = std::env::args().collect();
            let deploy_params = url_scheme::parse_deploy_args(&args);
            if deploy_params.is_some() {
                let state = app.state::<DeployState>();
                if let Ok(mut guard) = state.0.lock() {
                    *guard = deploy_params;
                };
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
